package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// tableReadyTimeout bounds how long startup waits for a new table to become active.
const tableReadyTimeout = 30 * time.Second

// EnsureTable creates a table keyed by a string attribute, unless it already exists, and waits until it is active.
// Setup is explicit (-create-tables) so a mistyped table name never silently creates a table in a real account.
func EnsureTable(ctx context.Context, client *dynamodb.Client, table, hashKey string) error {
	_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(table)})
	var missing *types.ResourceNotFoundException
	if err == nil {
		return nil
	}
	if !errors.As(err, &missing) {
		return fmt.Errorf("describe table %s: %w", table, err)
	}
	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:            aws.String(table),
		BillingMode:          types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{{AttributeName: aws.String(hashKey), AttributeType: types.ScalarAttributeTypeS}},
		KeySchema:            []types.KeySchemaElement{{AttributeName: aws.String(hashKey), KeyType: types.KeyTypeHash}},
	})
	var inUse *types.ResourceInUseException
	// Another instance starting at the same time may have created the table first; that is fine, we only need it to exist.
	if err != nil && !errors.As(err, &inUse) {
		return fmt.Errorf("create table %s: %w", table, err)
	}
	waiter := dynamodb.NewTableExistsWaiter(client)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(table)}, tableReadyTimeout); err != nil {
		return fmt.Errorf("wait for table %s: %w", table, err)
	}
	return nil
}
