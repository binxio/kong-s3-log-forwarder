CREATE EXTERNAL TABLE `kong_http_log`(
  `api` struct<created_at:bigint,headers:struct<host:array<string>>,hosts:array<string>,http_if_terminated:boolean,https_only:boolean,id:string,name:string,preserve_host:boolean,retries:int,strip_uri:boolean,upstream_connect_timeout:int,upstream_read_timeout:int,upstream_send_timeout:int,upstream_url:string,uris:array<string>> COMMENT '', 
  `authenticated_entity` struct<consumer_id:string,id:string> COMMENT '', 
  `client_ip` string COMMENT '', 
  `consumer` struct<created_at:bigint,id:string,username:string> COMMENT '', 
  `latencies` struct<kong:int,proxy:int,request:int> COMMENT '', 
  `request` struct<request_uri:string,size:string,uri:string> COMMENT '', 
  `response` struct<size:string,status:int> COMMENT '', 
  `started_at` bigint COMMENT '', 
  `tries` array<struct<balancer_latency:int,ip:string,port:int>> COMMENT '')
ROW FORMAT SERDE 
  'org.openx.data.jsonserde.JsonSerDe' 
LOCATION
  's3://kong-api-gateway-logs/';

