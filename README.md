# s3sync

s3 文件同步服务

应用部署实例较多,在不开通 sftp,ftp,且新增实例不能访问 git 仓库的情况下,为了让文件能自动同步,特编写本程序.

文件上传到 s3 之后,会触发一个 SQS 事件,本应用从 SQS 中获取消息,从 s3 中复制文件到本地

运行:  `s3sync --config=<配置文件>`




## 处理流程
s3sync 定时执行,从 指定的消息队列读取消息,对得到的消息进行处理.

从 sqs 中得到的 key 的样式如下:

```
test-folder/folder-01/folder-02/peccancy.html
test-folder/folder-01/folder-02/left.png
```

得到上述的 key 之后，需要跟配置文件中的 `sync->[key]` 做最大化的匹配,匹配成功则将当前 key 指向的值复制到 `sync->[value]` 指定的目录中,完全匹配的前缀会被忽略。

例如:

有配置 `test-folder/folder-01/ => /srv/web-pages/html/`

从 sqs 中读取到的 `object key`` 为 `test-folder/folder-01/folder-02/peccancy.html` 

则 `test-folder/folder-01/folder-02/peccancy.html` 中匹配的前缀 `test-folder/folder-01/` 忽略,变成 `folder-02/peccancy.html`,
然后将 `object key` 的文件复制到本地的 `/srv/web-pages/html/folder-02/peccancy.html` 目录中.

注: 从 sqs 中得到的 object key 不包含 s3 bucket name


## 配置文件样式

```json

{
	"queue":"transport-dev",
	"sync":[
    "test-folder/": "/tmp/xxx/local-store-dir",
    "test-folder/folder-01/": "/tmp/srv/local-store-dir",
    "a/": "b",
    "domain.com/path1/path2/": "/srv/web-pages/html/CmbcRecharge",
    "cc/": "b"
	]
}

```

## 文件上传事件


```json
{
    "Records": [
        {
            "eventVersion": "2.0",
            "eventSource": "aws:s3",
            "awsRegion": "cn-north-1",
            "eventTime": "2017-07-14T09:16:28.877Z",
            "eventName": "ObjectCreated:Put",
            "userIdentity": {
                "principalId": "AWS:AIDAPMPV34ZZL6EU6GYYW"
            },
            "requestParameters": {
                "sourceIPAddress": "1.119.128.6"
            },
            "responseElements": {
                "x-amz-request-id": "84143A72250ABA0B",
                "x-amz-id-2": "kj6un32mldro+auadL6f+OWw86fH6vjoGljot0mAoWV7VGu1yyJ+dBAh6mWius+H"
            },
            "s3": {
                "s3SchemaVersion": "1.0",
                "configurationId": "EventForTransport",
                "bucket": {
                    "name": "transport-dev",
                    "ownerIdentity": {
                        "principalId": "AWS:xxxxxxx"
                    },
                    "arn": "arn:aws:s3:::transport-dev"
                },
                "object": {
                    "key": "plink.exe",
                    "size": 545880,
                    "eTag": "528248ae133191c591ec6d12732f2cfd",
                    "sequencer": "0059688BECBD741D89"
                }
            }
        }
    ]
}

```


## 文件删除事件

```json
{
    "Records": [
        {
            "eventVersion": "2.0",
            "eventSource": "aws:s3",
            "awsRegion": "cn-north-1",
            "eventTime": "2017-07-14T09:06:40.490Z",
            "eventName": "ObjectRemoved:Delete",
            "userIdentity": {
                "principalId": "AWS:AIDAPMPV34ZZL6EU6GYYW"
            },
            "requestParameters": {
                "sourceIPAddress": "54.222.13.7"
            },
            "responseElements": {
                "x-amz-request-id": "8ADFAFDA4AAA0F4A",
                "x-amz-id-2": "GDKw54pQliHWDHkQQMP1QMJg9Y/pYSZGAZeBawwO2hUOTaklO9Ro6Bpzwhyxj0xNqs9ColpmFiQ="
            },
            "s3": {
                "s3SchemaVersion": "1.0",
                "configurationId": "EventForTransport",
                "bucket": {
                    "name": "transport-dev",
                    "ownerIdentity": {
                        "principalId": "AWS:xxxxxxx"
                    },
                    "arn": "arn:aws:s3:::transport-dev"
                },
                "object": {
                    "key": "xxxxx.html",
                    "sequencer": "00596889A0793F359F"
                }
            }
        }
    ]
}


```


## 创建目录事件

```json
{
    "Records": [
        {
            "eventVersion": "2.0",
            "eventSource": "aws:s3",
            "awsRegion": "cn-north-1",
            "eventTime": "2017-07-14T09:19:24.046Z",
            "eventName": "ObjectCreated:Put",
            "userIdentity": {
                "principalId": "AWS:AIDAPMPV34ZZL6EU6GYYW"
            },
            "requestParameters": {
                "sourceIPAddress": "54.222.11.3"
            },
            "responseElements": {
                "x-amz-request-id": "198665B125B981BF",
                "x-amz-id-2": "ba+y9Ir7Na+gAE8lVhX1FRioEbbmA8W3IpaaC2c6K5INJ6mBRTA+uT6JeF5JkuqMcyfqic7/V1U="
            },
            "s3": {
                "s3SchemaVersion": "1.0",
                "configurationId": "EventForTransport",
                "bucket": {
                    "name": "transport-dev",
                    "ownerIdentity": {
                        "principalId": "AWS:xxxxxxx"
                    },
                    "arn": "arn:aws:s3:::transport-dev"
                },
                "object": {
                    "key": "test-folder/",
                    "size": 0,
                    "eTag": "d41d8cd98f00b204e9800998ecf8427e",
                    "sequencer": "0059688C9BF506F8D3"
                }
            }
        }
    ]
}

````


## S3 事件配置

在 s3 bucket 中配置 event

**Event**

* ObjectDelete(All)
* ObjectCreate (All)
* Type SQS [queue name]


## SQS Policy

```json
{
  "Version": "2012-10-17",
  "Id": "arn:aws-cn:sqs:cn-north-1:xxxxxx:ms-transport-dev/SQSDefaultPolicy",
  "Statement": [
    {
      "Sid": "Sid1500023033663",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "SQS:SendMessage",
      "Resource": "arn:aws-cn:sqs:cn-north-1:xxxxxx:ms-transport-dev",
      "Condition": {
        "ArnLike": {
          "aws:SourceArn": "arn:aws-cn:s3:*:*:transport-dev"
        }
      }
    }
  ]
}
```


## 部署运行

在需要从 s3 中同步文件的 ec2 实例上运行本应用，可用 supervisor 作为守护进行运行或 cron 定时执行

**ec2 实例所需权限**

1. 从配置文件中指定的消息队列中获取消息
1. 从配置文件中指定的消息队列中删除消息
1. 从触发事件的 s3 中读取对象
1. 运行本程序的用户能读写配置文件中 `sync` 配置段中的本地存储路径


