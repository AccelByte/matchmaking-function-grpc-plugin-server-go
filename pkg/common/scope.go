// Copyright (c) 2024 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package common

import (
	"context"

	"github.com/sirupsen/logrus"

	"go.opentelemetry.io/otel"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	traceIdLogField = "traceID"
	tracerName      = "mm-server"
)

func ChildScopeFromRemoteScope(ctx context.Context, name string) *Scope {
	tracer := otel.Tracer(tracerName)
	tracerCtx, span := tracer.Start(ctx, name)
	traceID := span.SpanContext().TraceID().String()
	if traceID == "" || len(traceID) != 32 {
		traceID = getUUID()
	}

	return &Scope{
		Ctx:     tracerCtx,
		TraceID: traceID,
		span:    span,
		Log:     logrus.WithField(traceIdLogField, traceID),
	}
}

func NewRootScope(rootCtx context.Context, name string, abTraceID string) *Scope {
	tracer := otel.Tracer(name)
	ctx, span := tracer.Start(rootCtx, name)

	if abTraceID == "" || len(abTraceID) != 32 {
		abTraceID = getUUID()
	}

	scope := &Scope{
		Ctx:     ctx,
		TraceID: abTraceID,
		span:    span,
		Log:     logrus.WithField(traceIdLogField, abTraceID),
	}

	return scope
}

// Scope used as the envelope to combine and transport request-related information by the chain of function calls
type Scope struct {
	Ctx     context.Context
	TraceID string
	span    oteltrace.Span
	Log     *logrus.Entry
}

// Finish finishes current scope
func (s *Scope) Finish() {
	s.span.End()
}
