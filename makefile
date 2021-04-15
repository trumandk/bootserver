all:
	docker-compose stop
	docker-compose build
	docker-compose up --no-start
	docker-compose start
