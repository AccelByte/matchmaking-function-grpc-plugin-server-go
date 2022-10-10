// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"errors"
	"io"

	tp "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/sirupsen/logrus"

	"google.golang.org/grpc"

	"plugin-arch-grpc-server-go/pkg/pb"
)

type MatchFunctionServer struct {
	pb.UnimplementedMatchFunctionServer
}

type matchFunctionMakeMatchesServer struct {
	grpc.ServerStream
}

func (x *matchFunctionMakeMatchesServer) Send(m *pb.MatchResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *matchFunctionMakeMatchesServer) Recv() (*pb.MakeMatchesRequest, error) {
	m := new(pb.MakeMatchesRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}

	return m, nil
}

func (x *matchFunctionMakeMatchesServer) StreamMatches() error {
	for {
		in, err := x.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			logrus.Print("server. error receiving from stream: ", err.Error())

			return err
		}
		logrus.Printf("echoing message %q", in.RequestType)

		var tickets []*pb.Ticket
		var teams []*pb.Match_Team
		tickets = append(tickets, in.GetTicket())

		errSend := x.Send(&pb.MatchResponse{Match: &pb.Match{
			Tickets:           tickets,
			Teams:             teams,
			RegionPreferences: nil,
			MatchAttributes:   nil,
		}})
		if errSend != nil {
			return errSend
		}
	}
}

func (m *MatchFunctionServer) GetStatCodes(ctx context.Context, req *pb.GetStatCodesRequest) (*pb.StatCodesResponse, error) {
	codes := []string{"1", "2"}
	logrus.Infof("stat codes: %s", codes)

	return &pb.StatCodesResponse{Codes: codes}, nil
}

func (m *MatchFunctionServer) ValidateTickets(ctx context.Context, req *pb.ValidateTicketRequest) (*pb.ValidateTicketResponse, error) {
	logrus.Info("validate ticket")

	return &pb.ValidateTicketResponse{Valid: true}, nil
}

func (m *MatchFunctionServer) MakeMatches(server pb.MatchFunction_MakeMatchesServer) error {
	in, err := server.Recv()
	if err != nil {
		logrus.Errorf("error during stream Recv: %s", err)

		return err
	}

	_, ok := in.GetRequestType().(*pb.MakeMatchesRequest_Parameters)
	if !ok {
		logrus.Error("not a MakeMatchesRequest_Parameters type")

		return errors.New("expected parameters in the first message were not met")
	}

	ticket := &pb.Ticket{
		TicketId:         GenerateUUID(),
		MatchPool:        "bar",
		CreatedAt:        &tp.Timestamp{Seconds: 10},
		Players:          nil,
		TicketAttributes: nil,
		Latencies:        nil,
	}
	var tickets []*pb.Ticket
	tickets = append(tickets, ticket)
	match := &pb.Match{
		Tickets:           tickets,
		Teams:             nil,
		RegionPreferences: nil,
		MatchAttributes:   nil,
	}
	logrus.Infof("make match: %s", match)

	return nil
}
