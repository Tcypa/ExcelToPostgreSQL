# Excel to PostgreSQL Processor
A tool to process Excel files into PostgreSQL via a web UI and API, running in Docker.
## Run
```bash
   git clone https://github.com/Tcypa/ExcelToPostgreSQL
   cd ExcelToPostgreSQL
   docker-compose up --build .
```
if there is a need to mount a folder with files, then the path to the folder is specified in docker-compose.yaml app:
```yaml
  volumes:
      - ./data:/app/data
```
where ./data is path to your folder with xlsx files.
