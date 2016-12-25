#!/bin/bash

DATE="date --rfc-3339=ns"

sleep 20

while (true); do
	$DATE > /var/tmp/start
	sleep $((RANDOM % 30))
	$DATE > /var/tmp/end
	sleep 60
done
