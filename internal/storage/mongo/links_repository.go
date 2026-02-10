package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LinksRepository struct {
	coll *mongo.Collection
}

type linkDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Slug      string             `bson:"slug"`
	URL       string             `bson:"url"`
	Notes     string             `bson:"notes,omitempty"`
	APIKey    string             `bson:"apiKey,omitempty"`
	CreatedAt time.Time          `bson:"createdAt"`
	ExpiresAt *time.Time         `bson:"expiresAt,omitempty"`
	Clicks    int64              `bson:"clicks,omitempty"`
}

func NewLinksRepository(m *db.Mongo) (*LinksRepository, error) {
	repo := &LinksRepository{coll: m.Collection("links")}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := repo.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_slug"),
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

func (r *LinksRepository) Insert(ctx context.Context, link *links.Link) error {
	doc := linkDoc{
		Slug:      link.Slug,
		URL:       link.URL,
		Notes:     link.Notes,
		APIKey:    link.APIKey,
		CreatedAt: link.CreatedAt.UTC(),
		ExpiresAt: link.ExpiresAt,
	}

	_, err := r.coll.InsertOne(ctx, doc)
	if err == nil {
		return nil
	}

	if mongo.IsDuplicateKeyError(err) {
		return links.ErrSlugTaken
	}

	return err
}

func (r *LinksRepository) FindBySlug(ctx context.Context, slug string) (*links.Link, error) {
	var doc linkDoc
	err := r.coll.FindOne(ctx, bson.M{"slug": slug}).Decode(&doc)
	if err == nil {
		return mapLinkDoc(doc), nil
	}

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, links.ErrNotFound
	}

	return nil, err
}

func (r *LinksRepository) FindActiveBySlugAndIncClick(ctx context.Context, slug string, at time.Time) (*links.Link, error) {
	now := at.UTC()

	filter := bson.M{
		"slug": slug,
		"$or": bson.A{
			bson.M{"expiresAt": bson.M{"$exists": false}},
			bson.M{"expiresAt": nil},
			bson.M{"expiresAt": bson.M{"$gte": now}},
		},
	}

	update := bson.M{
		"$inc": bson.M{"clicks": 1},
	}

	var doc linkDoc
	err := r.coll.FindOneAndUpdate(
		ctx,
		filter,
		update,
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if err == nil {
		return mapLinkDoc(doc), nil
	}

	if errors.Is(err, mongo.ErrNoDocuments) {
		existing, findErr := r.FindBySlug(ctx, slug)
		if findErr == nil && existing != nil {
			return nil, links.ErrExpired
		}
		if findErr != nil {
			return nil, findErr
		}
		return nil, links.ErrNotFound
	}

	return nil, err
}

func mapLinkDoc(doc linkDoc) *links.Link {
	return &links.Link{
		Slug:      doc.Slug,
		URL:       doc.URL,
		Notes:     doc.Notes,
		APIKey:    doc.APIKey,
		CreatedAt: doc.CreatedAt,
		ExpiresAt: doc.ExpiresAt,
		Clicks:    doc.Clicks,
	}
}
