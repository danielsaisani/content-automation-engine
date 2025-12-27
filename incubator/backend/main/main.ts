import { SystemClock } from "@incubator/clock";
import { startApplication } from "./application";
import { parseApplicationConfig } from "./application-config";

const application = await startApplication(parseApplicationConfig(process.env), new SystemClock());

async function closeApplication() {
  void application.close()
  .then(() => {
    console.log("Application closed successfully.");
    process.exit(0);
  }).catch((error) => {
    console.error("Error while closing application:", error);
    process.exit(1);
  });
}

process.on("SIGTERM", () => {
    console.log("Received SIGTERM, shutting down application...");
    void closeApplication();
});

process.on("SIGINT", () => {
    console.log("Received SIGINT, shutting down application...");
    void closeApplication();
});