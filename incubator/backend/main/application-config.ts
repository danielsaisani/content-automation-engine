import { z } from 'zod';

export const ApplicationConfigSchema = z.object({
    // DATABASE
    POSTGRES_DB: z.string().min(1, "POSTGRES_DB is required"),
    POSTGRES_USER: z.string().min(1, "POSTGRES_USER is required"),
    POSTGRES_PASSWORD: z.string().min(1, "POSTGRES_PASSWORD is required"),
    POSTGRES_HOST: z.string().min(1, "POSTGRES_HOST is required"),
    POSTGRES_PORT: z.string().min(1, "POSTGRES_PORT is required"),
    POSTGRES_CA_CERTIFICATE: z.string().optional(),
});

export type ApplicationConfig = z.infer<typeof ApplicationConfigSchema>;

export const parseApplicationConfig = (value: unknown): ApplicationConfig => ApplicationConfigSchema.parse(value);