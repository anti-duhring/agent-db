package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// Compile-time interface check: DynamoDBRepository must implement ChatRepository.
var _ repository.ChatRepository = (*DynamoDBRepository)(nil)

const tableName = "chat_data"

// DynamoDBRepository implements ChatRepository using a single-table DynamoDB design.
type DynamoDBRepository struct {
	client *dynamodb.Client
	table  string
}

// Key encoding functions per locked decisions D-02 and D-03.

// userPK returns the partition key for conversation listing items.
// Format: USER#<partnerID>#<userID>
func userPK(partnerID, userID uuid.UUID) string {
	return fmt.Sprintf("USER#%s#%s", partnerID.String(), userID.String())
}

// convSK returns the sort key for conversation listing items.
// Format: CONV#<updatedAt RFC3339Nano>#<convID>
func convSK(updatedAt time.Time, convID uuid.UUID) string {
	return fmt.Sprintf("CONV#%s#%s", updatedAt.UTC().Format(time.RFC3339Nano), convID.String())
}

// convPK returns the partition key for conversation metadata and message items.
// Format: CONV#<convID>
func convPK(convID uuid.UUID) string {
	return fmt.Sprintf("CONV#%s", convID.String())
}

// msgSK returns the sort key for message items.
// Format: MSG#<createdAt RFC3339Nano>#<msgID>
func msgSK(createdAt time.Time, msgID uuid.UUID) string {
	return fmt.Sprintf("MSG#%s#%s", createdAt.UTC().Format(time.RFC3339Nano), msgID.String())
}

// convMetaRecord is the DynamoDB record for conversation metadata.
// PK=CONV#<conv_id>, SK=CONV#META
type convMetaRecord struct {
	PK             string `dynamodbav:"PK"`
	SK             string `dynamodbav:"SK"`
	ConvID         string `dynamodbav:"conv_id"`
	PartnerID      string `dynamodbav:"partner_id"`
	UserID         string `dynamodbav:"user_id"`
	CreatedAt      string `dynamodbav:"created_at"`
	UpdatedAt      string `dynamodbav:"updated_at"`
	UserPK         string `dynamodbav:"user_pk"`
	ConvListingSK  string `dynamodbav:"conv_listing_sk"`
}

// convListingRecord is the DynamoDB record for conversation listing.
// PK=USER#<partner>#<user>, SK=CONV#<updated_at>#<conv_id>
type convListingRecord struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	ConvID    string `dynamodbav:"conv_id"`
	PartnerID string `dynamodbav:"partner_id"`
	UserID    string `dynamodbav:"user_id"`
	CreatedAt string `dynamodbav:"created_at"`
	UpdatedAt string `dynamodbav:"updated_at"`
}

// msgRecord is the DynamoDB record for a message item.
// PK=CONV#<conv_id>, SK=MSG#<created_at>#<msg_id>
type msgRecord struct {
	PK             string `dynamodbav:"PK"`
	SK             string `dynamodbav:"SK"`
	MsgID          string `dynamodbav:"msg_id"`
	ConversationID string `dynamodbav:"conversation_id"`
	Role           string `dynamodbav:"role"`
	Content        string `dynamodbav:"content"`
	TokenCount     int    `dynamodbav:"token_count"`
	CreatedAt      string `dynamodbav:"created_at"`
}

// New creates a new DynamoDBRepository.
// If endpoint is non-empty, it configures the client to use that endpoint (for LocalStack testing).
func New(ctx context.Context, endpoint string) (*DynamoDBRepository, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion("us-east-1"),
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	clientOpts := []func(*dynamodb.Options){}
	if endpoint != "" {
		// LocalStack: use static credentials and override endpoint
		cfg.Credentials = credentials.NewStaticCredentialsProvider("test", "test", "")
		clientOpts = append(clientOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	client := dynamodb.NewFromConfig(cfg, clientOpts...)

	r := &DynamoDBRepository{
		client: client,
		table:  tableName,
	}

	if err := r.ensureTable(ctx); err != nil {
		return nil, fmt.Errorf("ensure table: %w", err)
	}

	return r, nil
}

// ensureTable creates the DynamoDB table if it doesn't already exist, then waits for it.
func (r *DynamoDBRepository) ensureTable(ctx context.Context) error {
	_, err := r.client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(r.table),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("PK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("SK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("PK"),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String("SK"),
				KeyType:       types.KeyTypeRange,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		var riu *types.ResourceInUseException
		if !errors.As(err, &riu) {
			return fmt.Errorf("create table: %w", err)
		}
		// Table already exists — no need to wait
		return nil
	}

	// Wait for table to become active
	waiter := dynamodb.NewTableExistsWaiter(r.client)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(r.table),
	}, 2*time.Minute); err != nil {
		return fmt.Errorf("wait for table: %w", err)
	}

	return nil
}

// Close is a no-op — the DynamoDB SDK client has no connection to close.
func (r *DynamoDBRepository) Close() {}

// CreateConversation creates a new conversation scoped to the given partner and user.
// Uses TransactWriteItems to atomically write the metadata item and the listing item.
func (r *DynamoDBRepository) CreateConversation(ctx context.Context, partnerID, userID uuid.UUID) (domain.Conversation, error) {
	id := uuid.New()
	now := time.Now().UTC()

	userPKVal := userPK(partnerID, userID)
	listingSK := convSK(now, id)
	nowStr := now.Format(time.RFC3339Nano)

	meta := convMetaRecord{
		PK:            convPK(id),
		SK:            "CONV#META",
		ConvID:        id.String(),
		PartnerID:     partnerID.String(),
		UserID:        userID.String(),
		CreatedAt:     nowStr,
		UpdatedAt:     nowStr,
		UserPK:        userPKVal,
		ConvListingSK: listingSK,
	}
	metaItem, err := attributevalue.MarshalMap(meta)
	if err != nil {
		return domain.Conversation{}, fmt.Errorf("marshal meta: %w", err)
	}

	listing := convListingRecord{
		PK:        userPKVal,
		SK:        listingSK,
		ConvID:    id.String(),
		PartnerID: partnerID.String(),
		UserID:    userID.String(),
		CreatedAt: nowStr,
		UpdatedAt: nowStr,
	}
	listingItem, err := attributevalue.MarshalMap(listing)
	if err != nil {
		return domain.Conversation{}, fmt.Errorf("marshal listing: %w", err)
	}

	_, err = r.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Put: &types.Put{
					TableName: aws.String(r.table),
					Item:      metaItem,
				},
			},
			{
				Put: &types.Put{
					TableName: aws.String(r.table),
					Item:      listingItem,
				},
			},
		},
	})
	if err != nil {
		return domain.Conversation{}, fmt.Errorf("transact write create conversation: %w", err)
	}

	return domain.Conversation{
		ID:        id,
		PartnerID: partnerID,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// AppendMessage atomically appends a message, rotates the conversation listing SK,
// and updates conversation metadata — all via TransactWriteItems (4 items per D-04).
func (r *DynamoDBRepository) AppendMessage(ctx context.Context, conversationID uuid.UUID, role domain.Role, content string) (domain.Message, error) {
	id := uuid.New()
	now := time.Now().UTC()
	tc := len(content) / 4
	nowStr := now.Format(time.RFC3339Nano)

	// Get current conversation metadata to find user_pk and old conv_listing_sk
	getResult, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: convPK(conversationID)},
			"SK": &types.AttributeValueMemberS{Value: "CONV#META"},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return domain.Message{}, fmt.Errorf("get conversation meta: %w", err)
	}
	if getResult.Item == nil {
		return domain.Message{}, fmt.Errorf("conversation not found: %s", conversationID)
	}

	var meta convMetaRecord
	if err := attributevalue.UnmarshalMap(getResult.Item, &meta); err != nil {
		return domain.Message{}, fmt.Errorf("unmarshal meta: %w", err)
	}

	oldListingSK := meta.ConvListingSK
	newListingSK := convSK(now, conversationID)

	// Build the 4 transact items
	msgRec := msgRecord{
		PK:             convPK(conversationID),
		SK:             msgSK(now, id),
		MsgID:          id.String(),
		ConversationID: conversationID.String(),
		Role:           string(role),
		Content:        content,
		TokenCount:     tc,
		CreatedAt:      nowStr,
	}
	msgItem, err := attributevalue.MarshalMap(msgRec)
	if err != nil {
		return domain.Message{}, fmt.Errorf("marshal message: %w", err)
	}

	// New listing record with updated timestamp
	newListing := convListingRecord{
		PK:        meta.UserPK,
		SK:        newListingSK,
		ConvID:    conversationID.String(),
		PartnerID: meta.PartnerID,
		UserID:    meta.UserID,
		CreatedAt: meta.CreatedAt,
		UpdatedAt: nowStr,
	}
	newListingItem, err := attributevalue.MarshalMap(newListing)
	if err != nil {
		return domain.Message{}, fmt.Errorf("marshal new listing: %w", err)
	}

	// Updated meta record
	updatedMeta := convMetaRecord{
		PK:            convPK(conversationID),
		SK:            "CONV#META",
		ConvID:        conversationID.String(),
		PartnerID:     meta.PartnerID,
		UserID:        meta.UserID,
		CreatedAt:     meta.CreatedAt,
		UpdatedAt:     nowStr,
		UserPK:        meta.UserPK,
		ConvListingSK: newListingSK,
	}
	updatedMetaItem, err := attributevalue.MarshalMap(updatedMeta)
	if err != nil {
		return domain.Message{}, fmt.Errorf("marshal updated meta: %w", err)
	}

	_, err = r.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			// 1. Put message item
			{
				Put: &types.Put{
					TableName: aws.String(r.table),
					Item:      msgItem,
				},
			},
			// 2. Delete old conversation listing item
			{
				Delete: &types.Delete{
					TableName: aws.String(r.table),
					Key: map[string]types.AttributeValue{
						"PK": &types.AttributeValueMemberS{Value: meta.UserPK},
						"SK": &types.AttributeValueMemberS{Value: oldListingSK},
					},
				},
			},
			// 3. Put new conversation listing item with updated timestamp
			{
				Put: &types.Put{
					TableName: aws.String(r.table),
					Item:      newListingItem,
				},
			},
			// 4. Put updated conversation metadata
			{
				Put: &types.Put{
					TableName: aws.String(r.table),
					Item:      updatedMetaItem,
				},
			},
		},
	})
	if err != nil {
		return domain.Message{}, fmt.Errorf("transact write append message: %w", err)
	}

	return domain.Message{
		ID:             id,
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		TokenCount:     tc,
		CreatedAt:      now,
	}, nil
}

// LoadWindow returns the last n messages from the specified conversation,
// ordered oldest-first. Queries DESC (ScanIndexForward=false) then reverses in-place.
func (r *DynamoDBRepository) LoadWindow(ctx context.Context, conversationID uuid.UUID, n int) ([]domain.Message, error) {
	keyCond := expression.Key("PK").Equal(expression.Value(convPK(conversationID))).
		And(expression.Key("SK").BeginsWith("MSG#"))

	expr, err := expression.NewBuilder().WithKeyCondition(keyCond).Build()
	if err != nil {
		return nil, fmt.Errorf("build expression: %w", err)
	}

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(r.table),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ScanIndexForward:          aws.Bool(false),
		Limit:                     aws.Int32(int32(n)),
		ConsistentRead:            aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}

	msgs := make([]domain.Message, 0, len(result.Items))
	for _, item := range result.Items {
		var rec msgRecord
		if err := attributevalue.UnmarshalMap(item, &rec); err != nil {
			return nil, fmt.Errorf("unmarshal message: %w", err)
		}

		msgID, err := uuid.Parse(rec.MsgID)
		if err != nil {
			return nil, fmt.Errorf("parse msg id: %w", err)
		}
		convID, err := uuid.Parse(rec.ConversationID)
		if err != nil {
			return nil, fmt.Errorf("parse conversation id: %w", err)
		}
		createdAt, err := time.Parse(time.RFC3339Nano, rec.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}

		msgs = append(msgs, domain.Message{
			ID:             msgID,
			ConversationID: convID,
			Role:           domain.Role(rec.Role),
			Content:        rec.Content,
			TokenCount:     rec.TokenCount,
			CreatedAt:      createdAt,
		})
	}

	// Reverse to produce oldest-first order (query returned DESC).
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nil
}

// ListConversations returns all conversations for the given (partnerID, userID) pair,
// sorted by most recently updated first. Returns an empty slice (not nil) when none match.
func (r *DynamoDBRepository) ListConversations(ctx context.Context, partnerID, userID uuid.UUID) ([]domain.Conversation, error) {
	keyCond := expression.Key("PK").Equal(expression.Value(userPK(partnerID, userID)))

	expr, err := expression.NewBuilder().WithKeyCondition(keyCond).Build()
	if err != nil {
		return nil, fmt.Errorf("build expression: %w", err)
	}

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(r.table),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ScanIndexForward:          aws.Bool(false),
		ConsistentRead:            aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("query conversations: %w", err)
	}

	convs := []domain.Conversation{}
	for _, item := range result.Items {
		var rec convListingRecord
		if err := attributevalue.UnmarshalMap(item, &rec); err != nil {
			return nil, fmt.Errorf("unmarshal conversation: %w", err)
		}

		convID, err := uuid.Parse(rec.ConvID)
		if err != nil {
			return nil, fmt.Errorf("parse conv id: %w", err)
		}
		pID, err := uuid.Parse(rec.PartnerID)
		if err != nil {
			return nil, fmt.Errorf("parse partner id: %w", err)
		}
		uID, err := uuid.Parse(rec.UserID)
		if err != nil {
			return nil, fmt.Errorf("parse user id: %w", err)
		}
		createdAt, err := time.Parse(time.RFC3339Nano, rec.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		updatedAt, err := time.Parse(time.RFC3339Nano, rec.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse updated_at: %w", err)
		}

		convs = append(convs, domain.Conversation{
			ID:        convID,
			PartnerID: pID,
			UserID:    uID,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}

	return convs, nil
}
