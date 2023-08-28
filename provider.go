package zerologgrpcprovider

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

type ZerologGrpcProvider interface {
	// UnaryInterceptor returns interceptor compatible with grpc api for provide zerolog logger
	UnaryInterceptor() grpc.UnaryServerInterceptor
	// StreamInterceptor returns interceptor compatible with grpc api for provide zerolog logger
	StreamInterceptor() grpc.StreamServerInterceptor
}

type zerologGrpcProvider struct {
	options *Options
}

func New(opts ...Option) (ZerologGrpcProvider, error) {
	defaultLogger := zerolog.New(os.Stdout)

	options := Options{
		logRequests:                  true,
		useRequestId:                 true,
		provideRequestFieldsToLogger: true,
		logErrors:                    true,
		requestLogger:                &defaultLogger,
	}

	for _, opt := range opts {
		err := opt(&options)
		if err != nil {
			return nil, fmt.Errorf("set option error: %w", err)
		}
	}

	return &zerologGrpcProvider{
		options: &options,
	}, nil
}

func (z *zerologGrpcProvider) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (
		resp interface{}, err error,
	) {
		loggerCtx := z.options.requestLogger.With().Bool(grpcUnaryInterceptorFieldName, true)
		if z.options.useRequestId {
			loggerCtx = z.loggerWithRequestId(loggerCtx)
		}

		logger := loggerCtx.Logger()
		loggerWithRequestFields := loggerCtx

		if z.options.provideRequestFieldsToLogger {
			loggerWithRequestFields = loggerCtx.Fields(map[string]interface{}{
				grpcRequestFieldName: req,
				grpcMethodFieldName:  info.FullMethod,
				grpcServerFieldName:  info.Server,
			})
		}

		if z.options.logRequests {
			logger := loggerWithRequestFields.Logger()
			(&logger).Debug().Msg("new unary request")
		}

		res, err := handler(contextWithLogger(ctx, &logger), req)
		if err != nil && z.options.logErrors {
			(&logger).Err(err).Msg("unary request error")

			return res, err
		}

		return res, err
	}
}

func (z *zerologGrpcProvider) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		loggerCtx := z.options.requestLogger.With().Bool(grpcUnaryInterceptorFieldName, false)
		if z.options.useRequestId {
			loggerCtx = z.loggerWithRequestId(loggerCtx)
		}

		if z.options.provideRequestFieldsToLogger {
			loggerCtx = loggerCtx.Fields(map[string]interface{}{
				grpcStreamInfoFieldName: info,
			})
		}

		logger := loggerCtx.Logger()

		if z.options.logRequests {
			(&logger).Debug().Msg("new stream request")
		}

		wrapper := serverStreamWrapper{
			ServerStream: ss,
			ctx:          contextWithLogger(ss.Context(), &logger),
		}

		err := handler(srv, &wrapper)
		if err != nil && z.options.logErrors {
			(&logger).Err(err).Msg("stream request error")
			return err
		}

		return err
	}
}

func (z *zerologGrpcProvider) loggerWithRequestId(ctx zerolog.Context) zerolog.Context {
	return ctx.Str(grpcRequestIdFieldName, uuid.NewString())
}
