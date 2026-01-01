# Logbook By Danny


9.8.S
- Project Outset
- [x] Decide Language (Most likely Go!)
- [x] Decide Architecture
- [x] Decide Infrastructure
    - Linode + GCP / GCP


- Fully TypeScript codebase
- Frontend
    - Dashboard for each tenant or user
        - Interface to change upload schedule
        - Interface to connect TikTok account


- Backend (Modular Monolith)
    - Modules
        - Scraper Service
        - Content Rendering Service
        - Scheduler Service
        - Tenant Config Service
        - Uploader Service
    - 1 Database with isolated views for each service (apart from Uploader service which has access to S3 like storage)

- Scraper Service
    - Runs on it's own cadence and pushes stories and shorts urls to a database
    - API
        - Retrieve stories by filters
            - Popularity
        - Retrieves most popular n shorts urls by category
    - Async Task Execution
        - Celery
        - Beat
            - Every hour, scrape new shorts

- The flow should be that the scheduler at 00:00AM UTC schedules the times for posts at random times in the day (UTC)
- Every tenant has a tiktok page that has a list of categories
- At the scheduled time of posting, the uploader service must make a request to the scraper service to retrieve a url to a short that matches the needs of the tenant
- The needs are:
    - This short must not have been uploaded already by this tenant (maybe not until some time has elapsed)
    - It is the most "popular" short in the set of shorts that are most "relevant" to the tenant





- Base dependencies such as temporal, the database client creation and a clock and their factories are handed to downstream factories to create the respective services