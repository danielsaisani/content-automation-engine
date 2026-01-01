import { Service } from "../../service";

type Url = string;

export interface ScraperServiceApi extends Service {
    getTopShortUrl(): Promise<Url>;
}