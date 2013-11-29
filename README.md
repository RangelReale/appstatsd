appstatsd
=========

Application logging and statistics, using MongoDB.

Uses the [statsd] protocol for statistics, and a custom udp protocol for logging.

This is an optionated library, that is, bucket names must follow a convention,
and data is collected in a certain way, which cannot be changed.

How it works - statistics
-------------------------

On receiving, statistics are consolidated in day, hour, and 15-minute intervals,
global and per-app.

The bucket name format is:

	appname.info1#param1#param2.info2#param1.infoX.field

where:

* appname: name of the sending application. If blank, statistics are not generated per app
* info1...infoN: information name. Each information level and is logged in a different collection, for example app.conn.proj.proc will generate these collections: conn, conn_proj and conn_proj_proc. Any statistics generated for this bucket will be saved to the 3 collections.
* #param1...#paramN: each information can have any number of parameters, that will be used as the key to aggregate statistics. For example, the buckets app.proc#sendmail and app.proc#sendsms will be aggretated on the same collection, but will have separated statistics, with a key named "proc" in addition to the default keys.

The real saved named is prefixed with the bucket type, c_ for COUNTER, t_ for TIMER, g_ for GAUGE. this must be taken in account when retrieving data.

For the TIMER and GAUGE parameter, an additional counter value is saved for each value, with the tc_ and gc_ prefix respectively.

How it works - logging
----------------------

Logging uses a simple udp message in this format:

	APP:LEVEL:MESSAGEID:MESSAGE
	
where:

* APP: name of the sending application.
* LEVEL: log level. 1=CRITICAL, 2=ERROR, 3=WARNING, 4=NOTICE, 5=INFO, 6=DEBUG.
* MESSAGEID: application defined message id, possible to be used in aggregated statistics. Not used be the application, just saved to the collection.
* MESSAGE: log message.

The message is saved on the "log" collection.

If Configuration.ErrorStatistics is true (the default), CRITICAL and ERROR are sent to COUNTER bucket APP.error.ct, and WARNING is sent to COUNTER bucket APP.error.wct.

Retrieving information
----------------------

Data can be retrieved using the built-in info webserver in the json format, or a simple chart for statistics.

All json retrieved data follow this format:

````json
{"error_code":0,"error_message":"","data":{"list":[...]}}
````

Log messages can be retrieved acessing:

	http://localhost:8127/log?amount=100
	
in this format:

````json
{"date":"2013-11-28T15:45:07.208-02:00","level":2,"app":"apdc-test","msg":"An errror"}
````

Statistics can be retrieved acessing:

	http://localhost:8127/stats/INFO?data=field1,field2&period=hour&output=json&app=APP&f_field=filtervalue
	
where:
	
	* data: REQUIRED. Comma-separated field names to retrieved, with type prefix as specified above.
	* period: day, hour, minute
	* output: json, chart
	* app: if present, uses the per-app statistics, else use the global ones.
	* f_FIELD: filter parameter if needed

json data is output in this format:

````json
{"date":"2013-11-28","hour":0,"minute":0,"t_dr":0,"tc_dr":0,"c_ct":20,}
````

The hour and minute fields depends on the period parameter.

Configuration file
------------------

The configuration file uses the [toml] format. See the appstatsd-default.conf files for an example.


Clients
-------

Any statsd client should be able to send statistics. By default the application uses the default statsd port (8125).


* Go: https://github.com/RangelReale/appstatsd-client


Dependencies
------------

	* gostatsd: https://github.com/kisielk/gostatsd
	* mgo: http://labix.org/mgo
	* gorilla/mux: http://github.com/gorilla/mux
	* plotinum: https://code.google.com/p/plotinum
	* epochdate: http://github.com/RangelReale/epochdate


Author
------

Rangel Reale



[statsd]: http://www.github.com/etsy/statsd
[toml]: https://github.com/mojombo/toml
