package server

import (
	"context"
	"errors"
	"log"
	"user-service/internal/service"

	pb "github.com/nexus-commerce/nexus-contracts-go/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedUserServiceServer
	svc service.Service
}

func NewServer(svc *service.Service) *Server {
	return &Server{
		svc: *svc,
	}
}

func (s *Server) Authenticate(ctx context.Context, r *pb.AuthenticateRequest) (*pb.AuthenticateResponse, error) {
	signedToken, err := s.svc.Authenticate(ctx, r.GetEmail(), r.GetPassword())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, service.ErrWrongPassword):
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.AuthenticateResponse{
		Token: signedToken,
	}, nil
}

func (s *Server) Register(ctx context.Context, r *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	user, err := s.svc.Register(ctx, r.GetFirstName(), r.GetLastName(), r.GetEmail(), r.GetPassword(), r.GetPasswordConfirmation())
	if err != nil {
		log.Println(err)
		switch {
		case errors.Is(err, service.ErrMismatchPassword):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		case errors.Is(err, service.ErrUserEmailAlreadyExists):
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, err
	}

	return &pb.RegisterResponse{
		User: &pb.User{
			Id:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		},
	}, nil
}

func (s *Server) GetProfile(ctx context.Context, _ *pb.GetProfileRequest) (*pb.GetProfileResponse, error) {
	userID, ok := ctx.Value("user-id").(int)
	if !ok {
		return nil, status.Error(codes.FailedPrecondition, "user id missing") // return FAILED_PRECONDITION status here as the system should never get into this state
	}

	userIDInt := int64(userID)

	user, err := s.svc.GetProfile(ctx, userIDInt)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.GetProfileResponse{
		User: &pb.User{
			Id:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		},
	}, nil
}
func (s *Server) UpdateProfile(ctx context.Context, r *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
	userID, ok := ctx.Value("user-id").(int)
	if !ok {
		return nil, status.Error(codes.FailedPrecondition, "user id missing") // return FAILED_PRECONDITION
	}

	userIDInt := int64(userID)

	p, err := s.svc.UpdateProfile(ctx, userIDInt, r)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.UpdateProfileResponse{
		User: &pb.User{
			Id:        p.ID,
			Email:     p.Email,
			FirstName: p.FirstName,
			LastName:  p.LastName,
		},
	}, nil
}
