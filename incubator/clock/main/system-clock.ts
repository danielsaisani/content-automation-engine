import { Clock } from "./clock";


/**
 * A clock implementation that returns the current system time.
 */
export class SystemClock implements Clock {
    public now(): Date {
        return new Date();
    }
}   