import { startApplication } from "./application";
import { parseApplicationConfig } from "./application-config";

const application = startApplication(parseApplicationConfig(process.env));