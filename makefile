all:
	docker-compose stop
	docker-compose build
	docker-compose up

split:
	split --bytes=47M lib.squash squash/lib.squash_
	#cat prefix* > bigfile
