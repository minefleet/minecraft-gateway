package dev.minefleet.api.gateway.networking.v1alpha1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class NetworkXDSGrpc {

  private NetworkXDSGrpc() {}

  public static final java.lang.String SERVICE_NAME = "network.v1alpha1.NetworkXDS";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest,
      dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse> getGetSnapshotMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetSnapshot",
      requestType = dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest.class,
      responseType = dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest,
      dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse> getGetSnapshotMethod() {
    io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest, dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse> getGetSnapshotMethod;
    if ((getGetSnapshotMethod = NetworkXDSGrpc.getGetSnapshotMethod) == null) {
      synchronized (NetworkXDSGrpc.class) {
        if ((getGetSnapshotMethod = NetworkXDSGrpc.getGetSnapshotMethod) == null) {
          NetworkXDSGrpc.getGetSnapshotMethod = getGetSnapshotMethod =
              io.grpc.MethodDescriptor.<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest, dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetSnapshot"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse.getDefaultInstance()))
              .setSchemaDescriptor(new NetworkXDSMethodDescriptorSupplier("GetSnapshot"))
              .build();
        }
      }
    }
    return getGetSnapshotMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static NetworkXDSStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkXDSStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkXDSStub>() {
        @java.lang.Override
        public NetworkXDSStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkXDSStub(channel, callOptions);
        }
      };
    return NetworkXDSStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static NetworkXDSBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkXDSBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkXDSBlockingV2Stub>() {
        @java.lang.Override
        public NetworkXDSBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkXDSBlockingV2Stub(channel, callOptions);
        }
      };
    return NetworkXDSBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static NetworkXDSBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkXDSBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkXDSBlockingStub>() {
        @java.lang.Override
        public NetworkXDSBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkXDSBlockingStub(channel, callOptions);
        }
      };
    return NetworkXDSBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static NetworkXDSFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkXDSFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkXDSFutureStub>() {
        @java.lang.Override
        public NetworkXDSFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkXDSFutureStub(channel, callOptions);
        }
      };
    return NetworkXDSFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void getSnapshot(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest request,
        io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetSnapshotMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service NetworkXDS.
   */
  public static abstract class NetworkXDSImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return NetworkXDSGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service NetworkXDS.
   */
  public static final class NetworkXDSStub
      extends io.grpc.stub.AbstractAsyncStub<NetworkXDSStub> {
    private NetworkXDSStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkXDSStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkXDSStub(channel, callOptions);
    }

    /**
     */
    public void getSnapshot(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest request,
        io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetSnapshotMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service NetworkXDS.
   */
  public static final class NetworkXDSBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<NetworkXDSBlockingV2Stub> {
    private NetworkXDSBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkXDSBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkXDSBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    public dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse getSnapshot(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetSnapshotMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service NetworkXDS.
   */
  public static final class NetworkXDSBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<NetworkXDSBlockingStub> {
    private NetworkXDSBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkXDSBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkXDSBlockingStub(channel, callOptions);
    }

    /**
     */
    public dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse getSnapshot(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetSnapshotMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service NetworkXDS.
   */
  public static final class NetworkXDSFutureStub
      extends io.grpc.stub.AbstractFutureStub<NetworkXDSFutureStub> {
    private NetworkXDSFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkXDSFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkXDSFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse> getSnapshot(
        dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetSnapshotMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_SNAPSHOT = 0;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_GET_SNAPSHOT:
          serviceImpl.getSnapshot((dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest) request,
              (io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getGetSnapshotMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotRequest,
              dev.minefleet.api.gateway.networking.v1alpha1.Api.GetSnapshotResponse>(
                service, METHODID_GET_SNAPSHOT)))
        .build();
  }

  private static abstract class NetworkXDSBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    NetworkXDSBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return dev.minefleet.api.gateway.networking.v1alpha1.Api.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("NetworkXDS");
    }
  }

  private static final class NetworkXDSFileDescriptorSupplier
      extends NetworkXDSBaseDescriptorSupplier {
    NetworkXDSFileDescriptorSupplier() {}
  }

  private static final class NetworkXDSMethodDescriptorSupplier
      extends NetworkXDSBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    NetworkXDSMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (NetworkXDSGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new NetworkXDSFileDescriptorSupplier())
              .addMethod(getGetSnapshotMethod())
              .build();
        }
      }
    }
    return result;
  }
}
