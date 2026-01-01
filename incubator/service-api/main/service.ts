import type { Clock } from "@incubator/clock";
import type { ServiceName, ServiceRpcClientFactory } from "./service-rpc";

export interface Service {
  close(): Promise<void>;
}

export interface ServiceDependencies {
  databaseFactory: any;
  logger: any;
  serviceRpcClientFactory: ServiceRpcClientFactory;
  clock: Clock;
  temporalConnectionFactory: any;
}

export interface ServiceFactory {
  serviceName(): ServiceName;
  launch(serviceDependencies: ServiceDependencies): Promise<Service>;
}
