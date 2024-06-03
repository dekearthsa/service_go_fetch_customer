package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/golang-jwt/jwt/v4"
)

type DBdata struct {
	AuthStatus bool     `json:"authStatus"`
	Email      string   `json:"email"`
	IsProduct  []string `json:"isProduct"`
	Tenan      string   `json:"tenan"`
	Type       string   `json:"type"`
}

type Claims struct {
	Data DBdata `json:"data"`
	jwt.RegisteredClaims
}

type CustomerData struct {
	CustomerID    string `json:"customerID"`
	CreateDate    int    `json:"createDate"`
	Email         string `json:"email"`
	CustomerType  string `json:"customerType"`
	CustomerTenan string `json:"customerTenan"`
}

func getFileFromS3(bucket, key string, region string) (string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("unable to load SDK config, %v", err)
	}

	client := s3.NewFromConfig(cfg)

	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := client.GetObject(context.TODO(), getObjectInput)
	if err != nil {
		return "", fmt.Errorf("failed to get file from S3, %v", err)
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read file content, %v", err)
	}

	return string(body), nil
}

func ValidateToken(tokens string) (int, string, string, error) {
	// fmt.Println("in ValidateToken")
	var REGION = "ap-southeast-1"
	var BUCKET = "cdk-hnb659fds-assets-058264531773-ap-southeast-1"
	var KEYFILE = "token.txt"
	setKey, err := getFileFromS3(BUCKET, KEYFILE, REGION)
	jwtKey := []byte(setKey)
	if err != nil {
		return 500, "Internal server error", "Internal server error", err
	}
	tokenString := strings.TrimPrefix(tokens, "Bearer ")
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtKey, nil
	})

	if err != nil {
		// fmt.Println("err ====> ", err)
		if err == jwt.ErrSignatureInvalid {
			return 401, "unauthorized", "unauthorized", err
		}
		return 401, "unauthorized", "unauthorized", err
	}

	if !token.Valid {
		return 401, "unauthorized", "unauthorized", err
	}

	return 200, claims.Data.Tenan, claims.Data.Type, nil
}

func FetchCustomerData(tenanName string) ([]CustomerData, error) {
	var tableName = tenanName + "_" + "demo_customer"
	var payload []CustomerData
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := dynamodb.New(sess)

	projectionExpression := "customerID, createDate, email, customerType, customerTenan"

	// Prepare the scan input
	params := &dynamodb.ScanInput{
		TableName:            aws.String(tableName),
		ProjectionExpression: aws.String(projectionExpression),
	}
	result, err := svc.Scan(params)
	if err != nil {
		fmt.Println("err scan DB ==> ", err)
		return payload, err
	}

	for _, item := range result.Items {
		el := CustomerData{}

		err = dynamodbattribute.UnmarshalMap(item, &el)
		if err != nil {
			fmt.Println("err unmarshalMap => ", err)
			return payload, err
		}

		var setData = CustomerData{
			CustomerID:    el.CustomerID,
			CreateDate:    el.CreateDate,
			Email:         el.Email,
			CustomerType:  el.CustomerType,
			CustomerTenan: el.CustomerTenan,
		}
		payload = append(payload, setData)
	}

	return payload, nil
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var token string = req.Headers["authorization"]
	status, tenan, userType, err := ValidateToken(token)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       fmt.Sprintf("error validate => %v", err),
		}, nil
	}
	if status != 200 {
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       "Invalid validate key",
		}, nil
	}
	if userType != "super_admin" && userType != "admin" && userType != "user" {
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       "Need permission.",
		}, nil
	}

	dataPayload, err := FetchCustomerData(tenan)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "internal server error",
		}, err
	}

	responseBody, err := json.Marshal(dataPayload)
	if err != nil {
		fmt.Println("error in json Marshal => ", err)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "internal server error",
		}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(responseBody),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func main() {
	lambda.Start(handler)
}
