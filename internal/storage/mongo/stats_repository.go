package mongo

import (
	"context"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ClickStatsRepository struct {
	coll *mongo.Collection
}

type clickDailyDoc struct {
	Slug  string `bson:"slug"`
	Date  string `bson:"date"` // YYYY-MM-DD (UTC)
	Count int64  `bson:"count"`
}

func NewClickStatsRepository(m *db.Mongo) (*ClickStatsRepository, error) {
	repo := &ClickStatsRepository{coll: m.Collection("clicks_daily")}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := repo.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}, {Key: "date", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_slug_date"),
		},
		{
			Keys:    bson.D{{Key: "slug", Value: 1}, {Key: "date", Value: -1}},
			Options: options.Index().SetName("slug_date_desc"),
		},
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *ClickStatsRepository) IncDaily(ctx context.Context, slug string, at time.Time) error {
	at = at.UTC()
	date := dateString(at)

	_, err := r.coll.UpdateOne(
		ctx,
		bson.M{"slug": slug, "date": date},
		bson.M{
			"$inc": bson.M{"count": 1},
			"$setOnInsert": bson.M{
				"slug": slug,
				"date": date,
			},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *ClickStatsRepository) GetDaily(ctx context.Context, slug string, from, to time.Time) ([]links.DailyCount, error) {
	from = from.UTC()
	to = to.UTC()

	cur, err := r.coll.Find(
		ctx,
		bson.M{
			"slug": slug,
			"date": bson.M{
				"$gte": dateString(from),
				"$lte": dateString(to),
			},
		},
		options.Find().SetSort(bson.D{{Key: "date", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []links.DailyCount
	for cur.Next(ctx) {
		var doc clickDailyDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, links.DailyCount{
			Date:  doc.Date,
			Count: doc.Count,
		})
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ClickStatsRepository) DeleteBySlug(ctx context.Context, slug string) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"slug": slug})
	return err
}

func dateString(t time.Time) string {
	return t.UTC().Format(time.DateOnly)
}
