// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package plugin_arch_grpc_server_go

import (
	"context"
	"fmt"
	"io"

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
			fmt.Printf("server: error receiving from stream: %v\n", err)
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

func (m *MatchFunctionServer) MakeMatches(pb.MatchFunction_MakeMatchesServer) error {
	ticket := &pb.Ticket{
		TicketId:         "foo",
		MatchPool:        "bar",
		CreatedAt:        nil,
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
