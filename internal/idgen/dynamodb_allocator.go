package idgen

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// counterName is the key of the single counter item in the id_counter table.
const counterName = "ids"

// DynamoDBAllocator is an Allocator backed by one counter item shared by every server, so their ranges never overlap.
type DynamoDBAllocator struct {
	client *dynamodb.Client
	table  string
}

// NewDynamoDBAllocator returns an allocator using the counter in table.
func NewDynamoDBAllocator(client *dynamodb.Client, table string) *DynamoDBAllocator {
	return &DynamoDBAllocator{client: client, table: table}
}

// NextRange adds size to the counter and returns its value from before the add. ADD is atomic in DynamoDB and UPDATED_OLD returns the
// pre-update value in the same call, so two servers can never be handed the same range. A missing item or attribute counts as 0.
func (a *DynamoDBAllocator) NextRange(ctx context.Context, size uint64) (uint64, error) {
	out, err := a.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(a.table),
		Key:                       map[string]types.AttributeValue{"name": &types.AttributeValueMemberS{Value: counterName}},
		UpdateExpression:          aws.String("ADD #n :size"),
		ExpressionAttributeNames:  map[string]string{"#n": "next"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":size": &types.AttributeValueMemberN{Value: strconv.FormatUint(size, 10)}},
		ReturnValues:              types.ReturnValueUpdatedOld,
	})
	if err != nil {
		return 0, fmt.Errorf("add to counter %s: %w", a.table, err)
	}
	old, ok := out.Attributes["next"].(*types.AttributeValueMemberN)
	if !ok {
		return 0, nil // First claim: the attribute did not exist yet.
	}
	start, err := strconv.ParseUint(old.Value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse counter %q: %w", old.Value, err)
	}
	return start, nil
}
