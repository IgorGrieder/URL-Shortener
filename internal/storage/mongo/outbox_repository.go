package mongo

import (
	"context"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	outboxCollectionName = "click_outbox"
	outboxStatusPending  = "pending"
	outboxStatusSent     = "sent"
)

type ClickOutboxRepository struct {
	coll *mongo.Collection
}

type outboxDoc struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	EventType     string             `bson:"eventType"`
	Slug          string             `bson:"slug"`
	OccurredAt    time.Time          `bson:"occurredAt"`
	TraceParent   string             `bson:"traceparent,omitempty"`
	TraceState    string             `bson:"tracestate,omitempty"`
	Baggage       string             `bson:"baggage,omitempty"`
	Status        string             `bson:"status"`
	Attempts      int                `bson:"attempts"`
	NextAttemptAt time.Time          `bson:"nextAttemptAt"`
	LastError     string             `bson:"lastError,omitempty"`
	CreatedAt     time.Time          `bson:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt"`
	SentAt        *time.Time         `bson:"sentAt,omitempty"`
}

type OutboxClickEvent struct {
	ID          primitive.ObjectID
	Slug        string
	OccurredAt  time.Time
	TraceParent string
	TraceState  string
	Baggage     string
	Attempts    int
}

func NewClickOutboxRepository(m *db.Mongo) (*ClickOutboxRepository, error) {
	repo := &ClickOutboxRepository{coll: m.Collection(outboxCollectionName)}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := repo.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "nextAttemptAt", Value: 1}, {Key: "createdAt", Value: 1}},
			Options: options.Index().SetName("status_nextAttempt_createdAt"),
		},
		{
			Keys:    bson.D{{Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("createdAt_desc"),
		},
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *ClickOutboxRepository) EnqueueClick(ctx context.Context, slug string, occurredAt time.Time) error {
	now := time.Now().UTC()
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	doc := outboxDoc{
		EventType:     "click.recorded",
		Slug:          slug,
		OccurredAt:    occurredAt.UTC(),
		TraceParent:   carrier.Get("traceparent"),
		TraceState:    carrier.Get("tracestate"),
		Baggage:       carrier.Get("baggage"),
		Status:        outboxStatusPending,
		Attempts:      0,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err := r.coll.InsertOne(ctx, doc)
	return err
}

func (r *ClickOutboxRepository) ListPending(ctx context.Context, now time.Time, limit int64) ([]OutboxClickEvent, error) {
	if limit <= 0 {
		limit = 1
	}

	cur, err := r.coll.Find(
		ctx,
		bson.M{
			"status":        outboxStatusPending,
			"nextAttemptAt": bson.M{"$lte": now.UTC()},
		},
		options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: 1}}).
			SetLimit(limit),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	events := make([]OutboxClickEvent, 0)
	for cur.Next(ctx) {
		var doc outboxDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		events = append(events, OutboxClickEvent{
			ID:          doc.ID,
			Slug:        doc.Slug,
			OccurredAt:  doc.OccurredAt,
			TraceParent: doc.TraceParent,
			TraceState:  doc.TraceState,
			Baggage:     doc.Baggage,
			Attempts:    doc.Attempts,
		})
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (r *ClickOutboxRepository) MarkSent(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now().UTC()
	_, err := r.coll.UpdateByID(
		ctx,
		id,
		bson.M{"$set": bson.M{
			"status":    outboxStatusSent,
			"updatedAt": now,
			"sentAt":    now,
			"lastError": "",
		}},
	)
	return err
}

func (r *ClickOutboxRepository) MarkRetry(ctx context.Context, id primitive.ObjectID, lastError string, nextAttemptAt time.Time) error {
	now := time.Now().UTC()
	_, err := r.coll.UpdateByID(
		ctx,
		id,
		bson.M{
			"$set": bson.M{
				"status":        outboxStatusPending,
				"lastError":     lastError,
				"nextAttemptAt": nextAttemptAt.UTC(),
				"updatedAt":     now,
			},
			"$inc": bson.M{"attempts": 1},
		},
	)
	return err
}
