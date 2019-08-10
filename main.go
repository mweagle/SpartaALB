package main

import (
	"context"
	"encoding/json"
	"fmt"
	_ "net/http/pprof" // include pprop
	"os"
	"strings"

	awsEvents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws/session"
	sparta "github.com/mweagle/Sparta"
	spartaCF "github.com/mweagle/Sparta/aws/cloudformation"
	spartaDecorators "github.com/mweagle/Sparta/decorator"
	gocf "github.com/mweagle/go-cloudformation"
	"github.com/sirupsen/logrus"
)

// ALB eligible lambda function
func helloWorld(ctx context.Context,
	albEvent awsEvents.ALBTargetGroupRequest) (awsEvents.ALBTargetGroupResponse, error) {

	logger, loggerOk := ctx.Value(sparta.ContextKeyLogger).(*logrus.Logger)
	if loggerOk {
		logger.Info("Accessing structured logger 🙌")
	}
	logger.WithFields(logrus.Fields{
		"Request": albEvent,
	}).Info("Request!")
	response, _ := json.Marshal(albEvent)

	return awsEvents.ALBTargetGroupResponse{
		StatusCode:        200,
		StatusDescription: fmt.Sprintf("200 OK"),
		Body:              string(response),
		IsBase64Encoded:   false,
		Headers:           map[string]string{},
	}, nil
}

// ALB eligible lambda function
func helloNewWorld(ctx context.Context,
	albEvent awsEvents.ALBTargetGroupRequest) (awsEvents.ALBTargetGroupResponse, error) {

	return awsEvents.ALBTargetGroupResponse{
		StatusCode:        200,
		StatusDescription: fmt.Sprintf("200 OK"),
		Body:              "Some other handler",
		IsBase64Encoded:   false,
		Headers:           map[string]string{},
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// Main
func main() {
	sess := session.Must(session.NewSession())
	awsName, awsNameErr := spartaCF.UserAccountScopedStackName("MyALBStack",
		sess)
	if awsNameErr != nil {
		fmt.Print("Failed to create stack name")
		os.Exit(1)
	}
	// 1. Lambda function
	lambdaFn, _ := sparta.NewAWSLambda("Hello World",
		helloWorld,
		sparta.IAMRoleDefinition{})
	lambdaFn2, _ := sparta.NewAWSLambda("Hello New World",
		helloNewWorld,
		sparta.IAMRoleDefinition{})

	// Create a SecurityGroup so that the ALB can accept connections
	sgResName := sparta.CloudFormationResourceName("ALBSecurityGroup", "ALBSecurityGroup")
	sgRes := &gocf.EC2SecurityGroup{
		GroupDescription: gocf.String("ALB Security Group"),
		SecurityGroupIngress: &gocf.EC2SecurityGroupIngressPropertyList{
			gocf.EC2SecurityGroupIngressProperty{
				IPProtocol: gocf.String("tcp"),
				FromPort:   gocf.Integer(80),
				ToPort:     gocf.Integer(80),
				CidrIP:     gocf.String("0.0.0.0/0"),
			},
		},
	}

	// The subnets are account specific. For this example they're being supplied
	// as an environment variable of the form TEST_SUBNETS=one,two,...
	subnetList := strings.Split(os.Getenv("TEST_SUBNETS"), ",")
	subnetIDs := make([]gocf.Stringable, len(subnetList))
	for eachIndex, eachSubnet := range subnetList {
		subnetIDs[eachIndex] = gocf.String(eachSubnet)
	}
	// Create the account-specific load balancer
	alb := &gocf.ElasticLoadBalancingV2LoadBalancer{
		Subnets:        gocf.StringList(subnetIDs...),
		SecurityGroups: gocf.StringList(gocf.GetAtt(sgResName, "GroupId")),
	}
	// The lambda function provided to the Decorator is the default handler for all
	// requests. See
	// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html#cfn-elasticloadbalancingv2-listener-defaultactions
	// for more information
	albDecorator, albDecoratorErr := spartaDecorators.NewApplicationLoadBalancerDecorator(alb,
		80,
		"HTTP",
		lambdaFn)
	if albDecoratorErr != nil {
		fmt.Print("Failed to create ALB Decorator")
		os.Exit(1)
	}
	// It's also possible to add conditionally evaluated routes if
	// there are multiple lambda lambda function target groups
	// for a given deployment.
	albDecorator.AddConditionalEntry(gocf.ElasticLoadBalancingV2ListenerRuleRuleCondition{
		Field: gocf.String("path-pattern"),
		PathPatternConfig: &gocf.ElasticLoadBalancingV2ListenerRulePathPatternConfig{
			Values: gocf.StringList(gocf.String("/newhello*")),
		},
	}, lambdaFn2)

	// Finally, tell the ALB decorator we have some additional resources that need to be
	// included in the CloudFormation template
	albDecorator.Resources[sgResName] = sgRes

	// Supply it to the WorkflowHooks and get going...
	workflowHooks := &sparta.WorkflowHooks{
		ServiceDecorators: []sparta.ServiceDecoratorHookHandler{
			albDecorator,
		},
	}

	// There are two lambda functions in this service
	var lambdaFunctions []*sparta.LambdaAWSInfo
	lambdaFunctions = append(lambdaFunctions, lambdaFn, lambdaFn2)

	err := sparta.MainEx(awsName,
		"Simple Sparta application that demonstrates how to make Lambda functions an ALB Target",
		lambdaFunctions,
		nil,
		nil,
		workflowHooks,
		false)
	if err != nil {
		os.Exit(1)
	}
}
