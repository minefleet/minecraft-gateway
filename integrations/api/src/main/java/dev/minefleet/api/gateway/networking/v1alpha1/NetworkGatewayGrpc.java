package dev.minefleet.api.gateway.networking.v1alpha1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * NetworkGateway lets external components (queues, matchmakers, gameserver
 * orchestrators) query where players currently are, without maintaining a
 * stream connection of their own.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class NetworkGatewayGrpc {

  private NetworkGatewayGrpc() {}

  public static final java.lang.String SERVICE_NAME = "network.v1alpha1.NetworkGateway";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest,
      dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse> getGetConnectionMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetConnection",
      requestType = dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest.class,
      responseType = dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest,
      dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse> getGetConnectionMethod() {
    io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest, dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse> getGetConnectionMethod;
    if ((getGetConnectionMethod = NetworkGatewayGrpc.getGetConnectionMethod) == null) {
      synchronized (NetworkGatewayGrpc.class) {
        if ((getGetConnectionMethod = NetworkGatewayGrpc.getGetConnectionMethod) == null) {
          NetworkGatewayGrpc.getGetConnectionMethod = getGetConnectionMethod =
              io.grpc.MethodDescriptor.<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest, dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetConnection"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse.getDefaultInstance()))
              .setSchemaDescriptor(new NetworkGatewayMethodDescriptorSupplier("GetConnection"))
              .build();
        }
      }
    }
    return getGetConnectionMethod;
  }

  private static volatile io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest,
      dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse> getGetPlayersForServerMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetPlayersForServer",
      requestType = dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest.class,
      responseType = dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest,
      dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse> getGetPlayersForServerMethod() {
    io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest, dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse> getGetPlayersForServerMethod;
    if ((getGetPlayersForServerMethod = NetworkGatewayGrpc.getGetPlayersForServerMethod) == null) {
      synchronized (NetworkGatewayGrpc.class) {
        if ((getGetPlayersForServerMethod = NetworkGatewayGrpc.getGetPlayersForServerMethod) == null) {
          NetworkGatewayGrpc.getGetPlayersForServerMethod = getGetPlayersForServerMethod =
              io.grpc.MethodDescriptor.<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest, dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetPlayersForServer"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse.getDefaultInstance()))
              .setSchemaDescriptor(new NetworkGatewayMethodDescriptorSupplier("GetPlayersForServer"))
              .build();
        }
      }
    }
    return getGetPlayersForServerMethod;
  }

  private static volatile io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest,
      dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse> getGetPlayersForServiceMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetPlayersForService",
      requestType = dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest.class,
      responseType = dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest,
      dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse> getGetPlayersForServiceMethod() {
    io.grpc.MethodDescriptor<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest, dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse> getGetPlayersForServiceMethod;
    if ((getGetPlayersForServiceMethod = NetworkGatewayGrpc.getGetPlayersForServiceMethod) == null) {
      synchronized (NetworkGatewayGrpc.class) {
        if ((getGetPlayersForServiceMethod = NetworkGatewayGrpc.getGetPlayersForServiceMethod) == null) {
          NetworkGatewayGrpc.getGetPlayersForServiceMethod = getGetPlayersForServiceMethod =
              io.grpc.MethodDescriptor.<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest, dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetPlayersForService"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse.getDefaultInstance()))
              .setSchemaDescriptor(new NetworkGatewayMethodDescriptorSupplier("GetPlayersForService"))
              .build();
        }
      }
    }
    return getGetPlayersForServiceMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static NetworkGatewayStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkGatewayStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkGatewayStub>() {
        @java.lang.Override
        public NetworkGatewayStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkGatewayStub(channel, callOptions);
        }
      };
    return NetworkGatewayStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static NetworkGatewayBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkGatewayBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkGatewayBlockingV2Stub>() {
        @java.lang.Override
        public NetworkGatewayBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkGatewayBlockingV2Stub(channel, callOptions);
        }
      };
    return NetworkGatewayBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static NetworkGatewayBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkGatewayBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkGatewayBlockingStub>() {
        @java.lang.Override
        public NetworkGatewayBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkGatewayBlockingStub(channel, callOptions);
        }
      };
    return NetworkGatewayBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static NetworkGatewayFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkGatewayFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkGatewayFutureStub>() {
        @java.lang.Override
        public NetworkGatewayFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkGatewayFutureStub(channel, callOptions);
        }
      };
    return NetworkGatewayFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * NetworkGateway lets external components (queues, matchmakers, gameserver
   * orchestrators) query where players currently are, without maintaining a
   * stream connection of their own.
   * </pre>
   */
  public interface AsyncService {

    /**
     */
    default void getConnection(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest request,
        io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetConnectionMethod(), responseObserver);
    }

    /**
     */
    default void getPlayersForServer(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest request,
        io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetPlayersForServerMethod(), responseObserver);
    }

    /**
     */
    default void getPlayersForService(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest request,
        io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetPlayersForServiceMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service NetworkGateway.
   * <pre>
   * NetworkGateway lets external components (queues, matchmakers, gameserver
   * orchestrators) query where players currently are, without maintaining a
   * stream connection of their own.
   * </pre>
   */
  public static abstract class NetworkGatewayImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return NetworkGatewayGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service NetworkGateway.
   * <pre>
   * NetworkGateway lets external components (queues, matchmakers, gameserver
   * orchestrators) query where players currently are, without maintaining a
   * stream connection of their own.
   * </pre>
   */
  public static final class NetworkGatewayStub
      extends io.grpc.stub.AbstractAsyncStub<NetworkGatewayStub> {
    private NetworkGatewayStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkGatewayStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkGatewayStub(channel, callOptions);
    }

    /**
     */
    public void getConnection(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest request,
        io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetConnectionMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void getPlayersForServer(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest request,
        io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetPlayersForServerMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void getPlayersForService(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest request,
        io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetPlayersForServiceMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service NetworkGateway.
   * <pre>
   * NetworkGateway lets external components (queues, matchmakers, gameserver
   * orchestrators) query where players currently are, without maintaining a
   * stream connection of their own.
   * </pre>
   */
  public static final class NetworkGatewayBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<NetworkGatewayBlockingV2Stub> {
    private NetworkGatewayBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkGatewayBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkGatewayBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    public dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse getConnection(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetConnectionMethod(), getCallOptions(), request);
    }

    /**
     */
    public dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse getPlayersForServer(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetPlayersForServerMethod(), getCallOptions(), request);
    }

    /**
     */
    public dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse getPlayersForService(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetPlayersForServiceMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service NetworkGateway.
   * <pre>
   * NetworkGateway lets external components (queues, matchmakers, gameserver
   * orchestrators) query where players currently are, without maintaining a
   * stream connection of their own.
   * </pre>
   */
  public static final class NetworkGatewayBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<NetworkGatewayBlockingStub> {
    private NetworkGatewayBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkGatewayBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkGatewayBlockingStub(channel, callOptions);
    }

    /**
     */
    public dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse getConnection(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetConnectionMethod(), getCallOptions(), request);
    }

    /**
     */
    public dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse getPlayersForServer(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetPlayersForServerMethod(), getCallOptions(), request);
    }

    /**
     */
    public dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse getPlayersForService(dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetPlayersForServiceMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service NetworkGateway.
   * <pre>
   * NetworkGateway lets external components (queues, matchmakers, gameserver
   * orchestrators) query where players currently are, without maintaining a
   * stream connection of their own.
   * </pre>
   */
  public static final class NetworkGatewayFutureStub
      extends io.grpc.stub.AbstractFutureStub<NetworkGatewayFutureStub> {
    private NetworkGatewayFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkGatewayFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkGatewayFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse> getConnection(
        dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetConnectionMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse> getPlayersForServer(
        dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetPlayersForServerMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse> getPlayersForService(
        dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetPlayersForServiceMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_CONNECTION = 0;
  private static final int METHODID_GET_PLAYERS_FOR_SERVER = 1;
  private static final int METHODID_GET_PLAYERS_FOR_SERVICE = 2;

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
        case METHODID_GET_CONNECTION:
          serviceImpl.getConnection((dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest) request,
              (io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse>) responseObserver);
          break;
        case METHODID_GET_PLAYERS_FOR_SERVER:
          serviceImpl.getPlayersForServer((dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest) request,
              (io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse>) responseObserver);
          break;
        case METHODID_GET_PLAYERS_FOR_SERVICE:
          serviceImpl.getPlayersForService((dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest) request,
              (io.grpc.stub.StreamObserver<dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse>) responseObserver);
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
          getGetConnectionMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionRequest,
              dev.minefleet.api.gateway.networking.v1alpha1.Api.GetConnectionResponse>(
                service, METHODID_GET_CONNECTION)))
        .addMethod(
          getGetPlayersForServerMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerRequest,
              dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServerResponse>(
                service, METHODID_GET_PLAYERS_FOR_SERVER)))
        .addMethod(
          getGetPlayersForServiceMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceRequest,
              dev.minefleet.api.gateway.networking.v1alpha1.Api.GetPlayersForServiceResponse>(
                service, METHODID_GET_PLAYERS_FOR_SERVICE)))
        .build();
  }

  private static abstract class NetworkGatewayBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    NetworkGatewayBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return dev.minefleet.api.gateway.networking.v1alpha1.Api.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("NetworkGateway");
    }
  }

  private static final class NetworkGatewayFileDescriptorSupplier
      extends NetworkGatewayBaseDescriptorSupplier {
    NetworkGatewayFileDescriptorSupplier() {}
  }

  private static final class NetworkGatewayMethodDescriptorSupplier
      extends NetworkGatewayBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    NetworkGatewayMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (NetworkGatewayGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new NetworkGatewayFileDescriptorSupplier())
              .addMethod(getGetConnectionMethod())
              .addMethod(getGetPlayersForServerMethod())
              .addMethod(getGetPlayersForServiceMethod())
              .build();
        }
      }
    }
    return result;
  }
}
