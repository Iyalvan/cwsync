package offsetstore

import (
	"fmt"
	"strconv"
	"time"

	"cwsync/internal/logger"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type DynamoDBOffsetStore struct {
	Client    *dynamodb.DynamoDB
	TableName string
}

// SaveOffset writes the offset item to DynamoDB.
func (d *DynamoDBOffsetStore) SaveOffset(key string, lastTimestamp int64) error {
	if d.Client == nil {
		return fmt.Errorf("dynamodb client not initialized")
	}
	if d.TableName == "" {
		return fmt.Errorf("dynamodb table name not configured")
	}
	_, err := d.Client.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(d.TableName),
		Item: map[string]*dynamodb.AttributeValue{
			"key":            {S: aws.String(key)},
			"last_timestamp": {N: aws.String(strconv.FormatInt(lastTimestamp, 10))},
		},
	})
	return err
}

func (d *DynamoDBOffsetStore) LoadOffset(key string, fallbackDuration time.Duration) int64 {
	if d.Client == nil {
		logger.Fatal.Fatalf("dynamodb client not initialized")
	}
	if d.TableName == "" {
		logger.Info.Printf("DynamoDB table name not configured, using fallback offset of %s", fallbackDuration)
		return time.Now().UTC().Add(-fallbackDuration).UnixMilli()
	}
	out, err := d.Client.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(d.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			"key": {S: aws.String(key)},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		logger.Fatal.Fatalf("Failed to load offset from DynamoDB: %v", err)
	}
	if out.Item == nil {
		logger.Info.Printf("Offset not found in DynamoDB, using default timestamp of %s", fallbackDuration)
		return time.Now().UTC().Add(-fallbackDuration).UnixMilli()
	}
	tsAttr, ok := out.Item["last_timestamp"]
	if !ok || tsAttr.N == nil {
		logger.Info.Printf("Offset attribute missing in DynamoDB item, using fallback of %s", fallbackDuration)
		return time.Now().UTC().Add(-fallbackDuration).UnixMilli()
	}
	lastTimestamp, err := strconv.ParseInt(*tsAttr.N, 10, 64)
	if err != nil {
		logger.Fatal.Fatalf("Failed to parse DynamoDB offset: %v", err)
	}
	return lastTimestamp
}
