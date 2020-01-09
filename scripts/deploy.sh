#!/bin/bash
heroku container:push judge-go
heroku container:release judge-go
