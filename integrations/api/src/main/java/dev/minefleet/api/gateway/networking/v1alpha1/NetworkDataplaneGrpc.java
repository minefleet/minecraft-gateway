package dev.minefleet.api.gateway.networking.v1alpha1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class NetworkDataplaneGrpc {

  private NetworkDataplaneGrpc() {}

  public static final java.lang.String SERVICE_NAME = "network.v1alpha1.NetworkDataplane";

  // Static method descriptors that strictly reflect the proto.
  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static NetworkDataplaneStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkDataplaneStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkDataplaneStub>() {
        @java.lang.Override
        public NetworkDataplaneStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkDataplaneStub(channel, callOptions);
        }
      };
    return NetworkDataplaneStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static NetworkDataplaneBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkDataplaneBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkDataplaneBlockingV2Stub>() {
        @java.lang.Override
        public NetworkDataplaneBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkDataplaneBlockingV2Stub(channel, callOptions);
        }
      };
    return NetworkDataplaneBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static NetworkDataplaneBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkDataplaneBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkDataplaneBlockingStub>() {
        @java.lang.Override
        public NetworkDataplaneBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkDataplaneBlockingStub(channel, callOptions);
        }
      };
    return NetworkDataplaneBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static NetworkDataplaneFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<NetworkDataplaneFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<NetworkDataplaneFutureStub>() {
        @java.lang.Override
        public NetworkDataplaneFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new NetworkDataplaneFutureStub(channel, callOptions);
        }
      };
    return NetworkDataplaneFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {
  }

  /**
   * Base class for the server implementation of the service NetworkDataplane.
   */
  public static abstract class NetworkDataplaneImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return NetworkDataplaneGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service NetworkDataplane.
   */
  public static final class NetworkDataplaneStub
      extends io.grpc.stub.AbstractAsyncStub<NetworkDataplaneStub> {
    private NetworkDataplaneStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkDataplaneStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkDataplaneStub(channel, callOptions);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service NetworkDataplane.
   */
  public static final class NetworkDataplaneBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<NetworkDataplaneBlockingV2Stub> {
    private NetworkDataplaneBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkDataplaneBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkDataplaneBlockingV2Stub(channel, callOptions);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service NetworkDataplane.
   */
  public static final class NetworkDataplaneBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<NetworkDataplaneBlockingStub> {
    private NetworkDataplaneBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkDataplaneBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkDataplaneBlockingStub(channel, callOptions);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service NetworkDataplane.
   */
  public static final class NetworkDataplaneFutureStub
      extends io.grpc.stub.AbstractFutureStub<NetworkDataplaneFutureStub> {
    private NetworkDataplaneFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected NetworkDataplaneFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new NetworkDataplaneFutureStub(channel, callOptions);
    }
  }


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
        .build();
  }

  private static abstract class NetworkDataplaneBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    NetworkDataplaneBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return dev.minefleet.api.gateway.networking.v1alpha1.Api.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("NetworkDataplane");
    }
  }

  private static final class NetworkDataplaneFileDescriptorSupplier
      extends NetworkDataplaneBaseDescriptorSupplier {
    NetworkDataplaneFileDescriptorSupplier() {}
  }

  private static final class NetworkDataplaneMethodDescriptorSupplier
      extends NetworkDataplaneBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    NetworkDataplaneMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (NetworkDataplaneGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new NetworkDataplaneFileDescriptorSupplier())
              .build();
        }
      }
    }
    return result;
  }
}
