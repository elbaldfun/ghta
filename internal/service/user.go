package service

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// UserInput is the create/update payload.
type UserInput struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type UserService struct {
	store *repository.Store
}

func NewUserService(store *repository.Store) *UserService {
	return &UserService{store: store}
}

func (s *UserService) Create(ctx context.Context, in UserInput) (*domain.User, error) {
	now := time.Now().UTC()
	role := in.Role
	if role == "" {
		role = "user"
	}
	u := domain.User{Email: in.Email, Name: in.Name, Role: role, CreatedAt: now, UpdatedAt: now}
	res, err := s.store.Users().InsertOne(ctx, u)
	if err != nil {
		return nil, err
	}
	u.ID = res.InsertedID.(primitive.ObjectID)
	return &u, nil
}

func (s *UserService) FindAll(ctx context.Context) ([]domain.User, error) {
	cur, err := s.store.Users().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	users := []domain.User{}
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (s *UserService) FindOne(ctx context.Context, id string) (*domain.User, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, badInput("invalid id")
	}
	var u domain.User
	err = s.store.Users().FindOne(ctx, bson.M{"_id": oid}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Update locates the user by id and returns the updated document (fixing the
// original's bug of passing an object where an id string was expected).
func (s *UserService) Update(ctx context.Context, id string, in UserInput) (*domain.User, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, badInput("invalid id")
	}
	set := bson.M{"updatedAt": time.Now().UTC()}
	if in.Email != "" {
		set["email"] = in.Email
	}
	if in.Name != "" {
		set["name"] = in.Name
	}
	if in.Role != "" {
		set["role"] = in.Role
	}
	res := s.store.Users().FindOneAndUpdate(ctx,
		bson.M{"_id": oid},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var u domain.User
	if err := res.Decode(&u); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *UserService) Remove(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return badInput("invalid id")
	}
	res, err := s.store.Users().DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}
