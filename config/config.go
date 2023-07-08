package config

var (
	S3Bucket      = "bulaba-jhg3d1vb" // TODO: Generate random bucket name
	Environment   = "dev"
	InitFilename = "bulaba.json"
)

type Config struct {
	Environment string	`json:"environment"`
	S3Bucket    string	`json:"s3_bucket"`
	ProjectName string	`json:"project_name"`
	AwsRegion	string	`json:"aws_region"`
	Profile  	string	`json:"profile"`
}