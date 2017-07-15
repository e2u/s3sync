package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
)

// 根据字符串长度对数组进行排序
type ByLength []string

func (s ByLength) Len() int           { return len(s) }
func (s ByLength) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s ByLength) Less(i, j int) bool { return len(s[i]) > len(s[j]) }

type Config struct {
	AWSQueue       string            `json:"aws-queue"`
	AWSRegion      string            `json:"aws-region"`
	Sync           map[string]string `json:"sync"`
	sortedSyncKeys []string          `json:"-"` // 用于存储已经根据长度排序的 key
}

// 根据 objectKey 获取本地存储路径,以最大化匹配原则来查找
// 返回这个 objectkey 在本地存储的完整路径
func (c *Config) GetLocalStorePath(objectKey string) string {
	// objectKey 从消息队列中得到的是 url encode 过的值
	objectKey, _ = url.QueryUnescape(objectKey)
	for _, k := range c.sortedSyncKeys {
		if strings.HasPrefix(objectKey, k) {
			local, ok := c.Sync[k]
			if !ok {
				return ""
			}
			if !strings.HasSuffix(local, "/") {
				local += "/"
			}
			logger.Printf("object key: %v match: %v local path: %v", objectKey, k, local)
			fullpath, err := filepath.Abs(local + objectKey[len(k):])
			if err != nil {
				logger.Println(err.Error())
				return ""
			}
			return fullpath
		}
	}
	return ""
}

type S3Event struct {
	Records []*struct {
		EventName string `json:"eventName"`
		S3        *struct {
			Object *struct {
				Key  string `json:"key"`
				Size int    `json:"size"`
			}
			Bucket *struct {
				Name string `json:"name"`
			}
		}
	}
}

var (
	configFile string
	config     *Config
	sess       *session.Session
	logger     = *log.New(os.Stdout, "s3sync:", log.Ldate|log.Ltime)
)

func init() {
	flag.StringVar(&configFile, "config", "config.json", "config file")
	flag.Parse()

	loadConfig()
	initAWSSession()
}

// 初始化配置文件
// 对配置文件做预处理
func loadConfig() {

	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(err.Error())
	}
	if err := json.Unmarshal(b, &config); err != nil {
		panic(err.Error())
	}

	// 根据 key 长度做排序处理
	for k, _ := range config.Sync {
		config.sortedSyncKeys = append(config.sortedSyncKeys, k)
	}
	sort.Stable(ByLength(config.sortedSyncKeys))
}

func initAWSSession() {
	creds := credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.EnvProvider{},
			&ec2rolecreds.EC2RoleProvider{
				Client: ec2metadata.New(session.New(aws.NewConfig().WithRegion(config.AWSRegion)), &aws.Config{Endpoint: aws.String("http://169.254.169.254/latest")}),
			},
		})
	awsCfg := aws.NewConfig().
		WithRegion(config.AWSRegion).
		WithCredentials(creds).
		WithCredentialsChainVerboseErrors(true).
		WithMaxRetries(3)

	sess = session.New(awsCfg)
}

func main() {

	// sess.Config.WithLogLevel(aws.LogDebugWithHTTPBody)
	sqss := sqs.New(sess)

	queueURL := getQueueUrl(config.AWSQueue, sqss)

	loopProc := func() {
		resp, err := sqss.ReceiveMessage(&sqs.ReceiveMessageInput{
			MaxNumberOfMessages: aws.Int64(10),
			QueueUrl:            queueURL,
		})
		if err != nil {
			logger.Printf("receive message error: %v", err.Error())
			return
		}
		logger.Printf("receive message count: %v", len(resp.Messages))
		for _, message := range resp.Messages {
			if err := procMessage(message); err != nil {
				logger.Println(err.Error())
				continue
			}
			sqss.DeleteMessage(&sqs.DeleteMessageInput{
				QueueUrl:      queueURL,
				ReceiptHandle: message.ReceiptHandle,
			})
		}
	}

	loop(loopProc)

}

// 处理单条消息
// 从 s3 下载内容并存储到本地目录
// 删除已经完成的消息
func procMessage(message *sqs.Message) error {
	s3event := S3Event{}
	if err := json.Unmarshal([]byte(*message.Body), &s3event); err != nil {
		return err
	}
	if len(s3event.Records) == 0 {
		return errors.New(fmt.Sprintf("s3event empty: %v", s3event))
	}
	record := s3event.Records[0]
	eventName := record.EventName
	objKey := record.S3.Object.Key
	bucketName := record.S3.Bucket.Name
	logger.Printf("event: %v bucket name: %v object key: %v", eventName, bucketName, objKey)
	// skip director
	if strings.HasSuffix(objKey, "/") {
		return nil
	}
	localPath := config.GetLocalStorePath(objKey)
	logger.Printf("local path: %v", localPath)

	switch eventName {
	case "ObjectCreated:Put": // 在 s3 中存储一个对象
		b, err := getObject(bucketName, objKey)
		if err != nil {
			return err
		}
		return saveObject(b, localPath)
	case "ObjectRemoved:Delete": // 在 s3 中删除一个对象
		return removeObject(localPath)
	}

	return nil
}

// 取队列 url
func getQueueUrl(queueName string, s *sqs.SQS) *string {
	resp, err := s.GetQueueUrl(&sqs.GetQueueUrlInput{QueueName: aws.String(queueName)})
	if err != nil {
		logger.Printf("get queue url error: %v", err.Error())
		return nil
	}
	return resp.QueueUrl
}

// 从 s3 上复制文件到本地目录
func getObject(bucketName, key string) ([]byte, error) {
	ts := s3.New(sess)
	resp, err := ts.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := &bytes.Buffer{}
	if _, err := bufio.NewReader(resp.Body).WriteTo(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// 在本地保存文件
// 如果要保存的文件的目录不存在,则创建
func saveObject(b []byte, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}
	out, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := out.Write(b); err != nil {
		return err
	}
	return nil
}

// 删除本地的文件
func removeObject(localPath string) error {
	return os.Remove(localPath)
}

func loop(f func()) {
	accessCount := 0
	for {
		accessCount++
		logger.Printf("Begin 0x%012x", accessCount)
		f()
		logger.Printf("End 0x%012x", accessCount)
	}
}
