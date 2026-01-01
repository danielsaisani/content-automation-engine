import { ScraperService, ServiceDependencies, ServiceFactory, ServiceName } from "@incubator/service-api"
import { ScraperServiceImpl } from "./scraper-service"

export const scraperServiceFactory: ServiceFactory = {
    serviceName(): ServiceName {
        return ServiceName.ScraperService
    },

    async launch({ logger, clock, databaseFactory, serviceRpcClientFactory, temporalConnectionFactory }: ServiceDependencies ): Promise<ScraperService.ScraperService> {
        return new ScraperServiceImpl();
    }
}