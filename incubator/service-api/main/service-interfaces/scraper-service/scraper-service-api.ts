import { Service } from "../../service";

type Url = string;

export interface ScraperService extends Service {
    getTopShortUrl(): Promise<Url>;
}