import {
    WebService,
    ScraperService,
}
from "../service-interfaces";

export enum ServiceName {
    WebService = "WebService",
    ScraperService = "ScraperService",
}

export type ServiceRpcClient<S extends ServiceName> = S extends ServiceName.WebService
    ? WebService.WebServiceApi
    : S extends ServiceName.ScraperService
    ? ScraperService.ScraperServiceApi
    : never;

export interface ServiceRpcClientFactory {
    createClient<S extends ServiceName>(serviceName: S): ServiceRpcClient<S>;
}