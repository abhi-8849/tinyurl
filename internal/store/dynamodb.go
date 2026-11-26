package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// item is the stored shape of a Mapping, named as in the url_mapping table. Its fields must stay in the same order and types as Mapping,
// because the two are converted directly.
type item struct {
	Code      string     `dynamodbav:"code"`
	LongURL   string     `dynamodbav:"long_url"`
	OwnerID   string     `dynamodbav:"owner_id"`
	CreatedAt time.Time  `dynamodbav:"created_at"`
	ExpiresAt time.Time  `dynamodbav:"expires_at"`
	DeletedAt *time.Time `dynamodbav:"deleted_at,omitempty"`
}

// DynamoDBRepository is a Repository backed by one DynamoDB table keyed by code. Rows are never purged, so a code is never reused.
type DynamoDBRepository struct {
	client *dynamodb.Client
	table  string
}

// NewDynamoDBRepository returns a repository using table.
func NewDynamoDBRepository(client *dynamodb.Client, table string) *DynamoDBRepository {
	return &DynamoDBRepository{client: client, table: table}
}

// Insert writes m only if its code is unused, so two creates racing for one alias cannot both win. It returns ErrConflict otherwise.
func (r *DynamoDBRepository) Insert(ctx context.Context, m Mapping) error {
	av, err := attributevalue.MarshalMap(item(m))
	if err != nil {
		return fmt.Errorf("marshal %s: %w", m.Code, err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.table), Item: av, ConditionExpression: aws.String("attribute_not_exists(code)"),
	})
	if isConditionFailed(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("put %s: %w", m.Code, err)
	}
	return nil
}

// Get reads code with an eventually consistent read, which costs half as much and is fine because links never change after create
// except for deletion, which the cache path already tolerates. It returns ErrNotFound if the code does not exist.
func (r *DynamoDBRepository) Get(ctx context.Context, code string) (Mapping, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(r.table), Key: key(code)})
	if err != nil {
		return Mapping{}, fmt.Errorf("get %s: %w", code, err)
	}
	if out.Item == nil {
		return Mapping{}, ErrNotFound
	}
	var it item
	if err := attributevalue.UnmarshalMap(out.Item, &it); err != nil {
		return Mapping{}, fmt.Errorf("unmarshal %s: %w", code, err)
	}
	return Mapping(it), nil
}

// MarkDeleted sets deleted_at on an existing row, or returns ErrNotFound. The condition stops an update from creating a row.
func (r *DynamoDBRepository) MarkDeleted(ctx context.Context, code string, at time.Time) error {
	av, err := attributevalue.Marshal(at)
	if err != nil {
		return fmt.Errorf("marshal time: %w", err)
	}
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.table),
		Key:                       key(code),
		UpdateExpression:          aws.String("SET deleted_at = :at"),
		ConditionExpression:       aws.String("attribute_exists(code)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":at": av},
	})
	if isConditionFailed(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("update %s: %w", code, err)
	}
	return nil
}

func key(code string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"code": &types.AttributeValueMemberS{Value: code}}
}

func isConditionFailed(err error) bool {
	var failed *types.ConditionalCheckFailedException
	return errors.As(err, &failed)
}
