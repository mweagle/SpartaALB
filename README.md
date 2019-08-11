# SpartaALB

Sparta-based application that demonstrates how to expose two Lambda functions as Application Load Balancer targets as described in the [documentation](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/lambda-functions.html).

1. [Install Go](https://golang.org/doc/install)
1. `go get github.com/mweagle/SpartaALB`
1. `cd ./SpartaHelloWorld`
1. `TEST_SUBNETS=id1,id2 go run main.go provision --s3Bucket YOUR_S3_BUCKET`
1. Visit the AWS Console and test your function!