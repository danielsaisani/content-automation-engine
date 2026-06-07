// Package api contains all the contracts that bespoke service dependencies must implement and conform to in order to be usable by the services. Communication between components should be via these contracts so that components remain swappable and there is no unnecessary tight coupling.
// Interactions between services themselves should _also_ be via these contracts so that the services are not dependent on each other, but rather on an abstraction of what the other services do. This is particularly useful when it comes to accessing resources that are maintained by a specific service.
// Another way to think of this package is that this represents the inter-module boundaries and API's
package api
