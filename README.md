# Excel to PostgreSQL Processor
A tool to process Excel files into PostgreSQL via a web UI and API, running in Docker.
## Run
```bash
   git clone https://github.com/Tcypa/ExcelToPostgreSQL
   cd ExcelToPostgreSQL
   docker-compose up --build .
```
if there is a need to mount a folder with files, then the path to the folder is specified in docker-compose.yaml app service:
```yaml
  volumes:
      - ./data:/app/data
```
where ```./data ``` is path to your folder with xlsx files.

## Using
at ```localhost:3000```, you have a web page where you need to specify the name of the file to transfer to the previously mounted folder, the data for connecting to the database, as well as the interval/run once for processing this file, and ignorant sheets which don't need to process in this xlsx file
Also you have pgAdmin at ```localhost:9090``` to manage Postgres.

## Warning
This application exposes PostgreSQL credentials in the UI input and logs. Use with caution in production—consider securing the API and masking sensitive data properly.
