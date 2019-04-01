package bom

// BOM Mongodb query builder of (go.mongodb.org/mongo-driver)

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"math"
	"strings"
	"time"
)

type (
	Bom struct {
		client          *mongo.Client
		dbName          string
		dbCollection    string
		queryTimeout    time.Duration
		condition       interface{}
		whereConditions []map[string]interface{}
		orConditions    []map[string]interface{}
		inConditions    []map[string]interface{}
		notConditions   []map[string]interface{}
		pagination      *Pagination
		limit           *Limit
		sort            *Sort
	}
	Pagination struct {
		TotalCount  int32
		TotalPages  int32
		CurrentPage int32
		Size        int32
	}
	Sort struct {
		Field string
		Type  string
	}
	Limit struct {
		Page int32
		Size int32
	}
	Size   int32
	Option func(*Bom) error
)

const (
	DefaultQueryTimeout = 5 * time.Second
	DefaultSize         = 20
)

var (
	mType = map[string]int32{"asc": 1, "desc": -1}
)

func New(options ...Option) (*Bom, error) {
	b := &Bom{
		queryTimeout: DefaultQueryTimeout,
		pagination: &Pagination{
			Size: DefaultSize,
		},
		limit: &Limit{Page: 1, Size: DefaultSize},
	}
	for _, option := range options {
		if err := option(b); err != nil {
			return nil, err
		}
	}
	if b.client == nil {
		return nil, fmt.Errorf("mondodb client is required")
	}
	return b, nil
}

func ToObj(id string) primitive.ObjectID {
	objectID, _ := primitive.ObjectIDFromHex(id)
	return objectID
}

func ToObjects(ids []string) []primitive.ObjectID {
	var objectIds []primitive.ObjectID
	for _, id := range ids {
		objectId, _ := primitive.ObjectIDFromHex(id)
		objectIds = append(objectIds, objectId)
	}
	return objectIds
}

func SetMongoClient(client *mongo.Client) Option {
	return func(b *Bom) error {
		b.client = client
		return nil
	}
}

func SetDatabaseName(dbName string) Option {
	return func(b *Bom) error {
		b.dbName = dbName
		return nil
	}
}

func SetCollection(collection string) Option {
	return func(b *Bom) error {
		b.dbCollection = collection
		return nil
	}
}

func SetQueryTimeout(time time.Duration) Option {
	return func(b *Bom) error {
		b.queryTimeout = time
		return nil
	}
}

func (b *Bom) WithDB(dbName string) *Bom {
	b.dbName = dbName
	return b
}

func (b *Bom) WithColl(collection string) *Bom {
	b.dbCollection = collection
	return b
}

func (b *Bom) WithTimeout(time time.Duration) *Bom {
	b.queryTimeout = time
	return b
}

func (b *Bom) WithCondition(condition interface{}) *Bom {
	b.condition = condition
	return b
}

func (b *Bom) WithLimit(limit *Limit) *Bom {
	b.limit = limit
	return b
}

func (b *Bom) WithSort(sort *Sort) *Bom {
	b.sort = sort
	return b
}

func (b *Bom) Where(field string, value interface{}) *Bom {
	b.whereConditions = append(b.whereConditions, map[string]interface{}{"field": field, "value": value})
	return b
}

func (b *Bom) Not(field string, value interface{}) *Bom {
	b.notConditions = append(b.notConditions, map[string]interface{}{"field": field, "value": value})
	return b
}

func (b *Bom) InWhere(field string, value interface{}) *Bom {
	b.inConditions = append(b.inConditions, map[string]interface{}{"field": field, "value": value})
	return b
}

func (b *Bom) OrWhere(field string, value interface{}) *Bom {
	b.orConditions = append(b.orConditions, map[string]interface{}{"field": field, "value": value})
	return b
}

func (b *Bom) buildCondition() interface{} {
	result := make(primitive.M)
	if len(b.whereConditions) > 0 {
		var query []primitive.M
		for _, cnd := range b.whereConditions {
			field := cnd["field"]
			value := cnd["value"]
			query = append(query, primitive.M{field.(string): value})
		}
		result["$and"] = query
	}
	if len(b.orConditions) > 0 {
		var query []primitive.M
		for _, cnd := range b.orConditions {
			field := cnd["field"]
			value := cnd["value"]
			query = append(query, primitive.M{field.(string): value})
		}
		result["$or"] = query
	}
	if len(b.inConditions) > 0 {
		for _, cnd := range b.inConditions {
			field := cnd["field"]
			value := cnd["value"]
			result[field.(string)] = primitive.M{"$in": value}
		}
	}
	return result
}

func (b *Bom) query() *mongo.Collection {
	return b.client.Database(b.dbName).Collection(b.dbCollection)
}

func (b *Bom) getTotalPages() int32 {
	d := float64(b.pagination.TotalCount) / float64(b.pagination.Size)
	if d < 0 {
		d = 1
	}
	return int32(math.Ceil(d))
}

func (b *Bom) getPagination(total int32, page int32, size int32) *Pagination {
	b.pagination.TotalCount = total
	if page > 0 {
		b.pagination.CurrentPage = page
	}
	if size > 0 {
		b.pagination.Size = size
	}
	b.pagination.TotalPages = b.getTotalPages()
	return b.pagination
}

func (b *Bom) calculateOffset(page, size int32) (limit int32, offset int32) {
	limit = b.limit.Size
	if page == 0 {
		page = 1
	}
	if size > 0 {
		limit = size
	}
	o := float64(page-1) * float64(limit)
	offset = int32(math.Ceil(o))
	return
}

func (b *Bom) getSort(sort *Sort) (map[string]interface{}, bool) {
	sortMap := make(map[string]interface{})
	if len(sort.Field) > 0 {
		sortMap[strings.ToLower(sort.Field)] = 1
		if len(sort.Type) > 0 {
			if val, ok := mType[strings.ToLower(sort.Type)]; ok {
				sortMap[strings.ToLower(sort.Type)] = val
			}
		}
		return sortMap, true
	}
	return sortMap, false
}

func (b *Bom) getCondition() interface{} {
	if b.condition != nil {
		return b.condition
	}
	bc := b.buildCondition()
	if bc != nil {
		if val, ok := bc.(primitive.M); ok {
			return val
		}
	}
	return primitive.M{}
}

func (b *Bom) UpdateOne(update interface{}) (*mongo.UpdateResult, error) {
	ctx, _ := context.WithTimeout(context.Background(), DefaultQueryTimeout)
	res, err := b.query().UpdateOne(ctx, b.getCondition(), update)
	return res, err
}

func (b *Bom) InsertOne(document interface{}) (*mongo.InsertOneResult, error) {
	ctx, _ := context.WithTimeout(context.Background(), DefaultQueryTimeout)
	return b.query().InsertOne(ctx, document)
}

func (b *Bom) FindOne(callback func(s *mongo.SingleResult) error) error {
	ctx, _ := context.WithTimeout(context.Background(), DefaultQueryTimeout)
	s := b.query().FindOne(ctx, b.getCondition())
	return callback(s)
}

func (b *Bom) FindOneAndDelete() *mongo.SingleResult {
	ctx, _ := context.WithTimeout(context.Background(), DefaultQueryTimeout)
	return b.query().FindOneAndDelete(ctx, b.getCondition())
}

func (b *Bom) ListWithPagination(callback func(cursor *mongo.Cursor) error) (*Pagination, error) {
	ctx, _ := context.WithTimeout(context.Background(), DefaultQueryTimeout)
	findOptions := options.Find()
	limit, offset := b.calculateOffset(b.limit.Page, b.limit.Size)
	findOptions.SetLimit(int64(limit)).SetSkip(int64(offset))
	if sm, ok := b.getSort(b.sort); ok {
		findOptions.SetSort(sm)
	}
	condition := b.getCondition()
	count, err := b.query().CountDocuments(ctx, condition)
	if err != nil {
		return &Pagination{}, err
	}
	cur, err := b.query().Find(ctx, condition, findOptions)
	if err != nil {
		return &Pagination{}, err
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		err = callback(cur)
	}
	if err := cur.Err(); err != nil {
		return &Pagination{}, err
	}
	pagination := b.getPagination(int32(count), b.limit.Page, b.limit.Size)
	return pagination, err
}

func (b *Bom) List(callback func(cursor *mongo.Cursor) error) error {
	ctx, _ := context.WithTimeout(context.Background(), DefaultQueryTimeout)
	cur, err := b.query().Find(ctx, b.getCondition())
	if err != nil {
		return err
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		err = callback(cur)
	}
	if err := cur.Err(); err != nil {
		return err
	}
	return err
}