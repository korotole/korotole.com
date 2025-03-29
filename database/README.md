# korotole.com

## Database (MySQL)

- check contents: 
    - `docker exec -it mysql bash`
    - `mysql -u root -p$MYSQL_ROOT_PASSWORD`
    - `SHOW DATABASES;`
    - `USE database_name;`
    - `SHOW TABLES;`
    - `SELECT * FROM table_name;`
- clear contents:
    - `docker-compose down`
    - `docker volume ls`
    - `docker volume rm <project_folder>_mysql_data`, or elsewise `docker volume prune` do remove **all** unused volumes