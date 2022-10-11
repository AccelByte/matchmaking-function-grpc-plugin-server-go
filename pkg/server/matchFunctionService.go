// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"errors"
	"sync"

	"github.com/sirupsen/logrus"

	"plugin-arch-grpc-server-go/pkg/pb"
)

type MatchFunctionServer struct {
	pb.UnimplementedMatchFunctionServer
	MatchMaker MatchLogic
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

	mrpT, ok := in.GetRequestType().(*pb.MakeMatchesRequest_Parameters)
	if !ok {
		logrus.Error("not a MakeMatchesRequest_Parameters type")

		return errors.New("expected parameters in the first message were not met")
	}

	rules, err := m.MatchMaker.RulesFromJSON(mrpT.Parameters.Rules.Json)
	if err != nil {
		logrus.Errorf("could not get rules from json: %s", err)

		return err
	}
	logrus.Infof("rules: %s", rules)

	resultChan := m.MatchMaker.MakeMatches(rules)

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range resultChan {
			logrus.Info("creating a match")
			match := &pb.Match{
				Tickets:           result.Tickets,
				RegionPreferences: nil,
				MatchAttributes:   nil,
			}
			resp := pb.MatchResponse{Match: match}
			if sErr := server.Send(&resp); err != nil {
				logrus.Errorf("error on server send: %s", sErr)

				return
			}
		}
	}()
	wg.Wait()

	logrus.Info("make match")

	return nil
}
