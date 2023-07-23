package cloud

const (
	awsDocs         = "https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials"
	awsAssumePolicy = `{
		"Version": "2012-10-17",
		"Statement": [
	  	{
			"Sid": "",
			"Effect": "Allow",
			"Principal": {
		  	"Service": [
				"apigateway.amazonaws.com",
				"lambda.amazonaws.com",
				"events.amazonaws.com"
		  	]
			},
			"Action": "sts:AssumeRole"
	  	}
		]
  	}`

	awsAttachPolicy = `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"logs:*"
				],
				"Resource": "arn:aws:logs:*:*:*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"lambda:InvokeFunction"
				],
				"Resource": [
					"*"
				]
			},
			{
				"Effect": "Allow",
				"Action": [
					"xray:PutTraceSegments",
					"xray:PutTelemetryRecords"
				],
				"Resource": [
					"*"
				]
			},
			{
				"Effect": "Allow",
				"Action": [
					"ec2:AttachNetworkInterface",
					"ec2:CreateNetworkInterface",
					"ec2:DeleteNetworkInterface",
					"ec2:DescribeInstances",
					"ec2:DescribeNetworkInterfaces",
					"ec2:DetachNetworkInterface",
					"ec2:ModifyNetworkInterfaceAttribute",
					"ec2:ResetNetworkInterfaceAttribute"
				],
				"Resource": "*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"s3:*"
				],
				"Resource": "arn:aws:s3:::*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"kinesis:*"
				],
				"Resource": "arn:aws:kinesis:*:*:*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"sns:*"
				],
				"Resource": "arn:aws:sns:*:*:*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"sqs:*"
				],
				"Resource": "arn:aws:sqs:*:*:*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"dynamodb:*"
				],
				"Resource": "arn:aws:dynamodb:*:*:*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"route53:*"
				],
				"Resource": "*"
			}
		]
	}`
)

type Platform interface {
	Deploy(string)
	CreateFunctionEntry(string)
	CheckPermissions()
}

type CloudConfig interface {
	ToJson()
	GetFunctionName() string
	GetRuntime() string
	GetBucket() string
}
