import { SystemClock } from "@incubator/clock";
import { startApplication } from "./application";
import { parseApplicationConfig } from "./application-config";

const application = await startApplication(parseApplicationConfig(process.env), new SystemClock());