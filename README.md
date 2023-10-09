# smock - another data mocker

### http server
市面上多数的http mock server是为WEB前端开发测试开发的，基本都严格遵循REST API的规范，可是现实中，服务端的开发中测试外部API碰到的情况却有点一言难尽，经常遇到API全部都定义成POST的情况，所以就有了本项目。

本项目的http server的配置默认还是基于REST规范的，但是也可以根据情况自由配置行为，比如通过POST/DELETE获取数据，通过GET设置数据，以应对特殊的情况

#### 配置文件
有默认值的项目都可以不配置
``` toml
# 配置语法是toml
# 监听端口 默认8080
port=8080
# 开启动态路由 post的数据会上传到默认uri对应的文件, get会按照路径动态获取文件，文件类型由Content-Type决定, 默认是true
dynamic_route=true
# db根目录 默认是http-server-root
db_root="sample-http-root"
# consul config
# consul_api_base="http://127.0.0.1:8500"
# consul_service_name="test-service"
# 默认是127.0.0.1
# consul_service_host="127.0.0.1"

[r1]
# get single data
path="/get/single/"
# method="get"
# action="r"
# format="json"
single=true

[r2]
# support path variable
path="/get/list"
# field project
fields=["id", "name"]
# don't use [] if there is only ONE datum
unique_not_list=true

[r7]
path="/post/list/data_with_ids"
method="post"
# action="a"
id=["id1", "id2"]
file="/post/list/data_with_2_id.json"

[r9]
path="/post/list/data_with_ids"
method="delete"
action="d"
file="/post/list/data_with_2_id.json"

```

配置主要包括2部分：基本配置和静态路由

**基本配置**

1. 端口
   
   http启动使用的端口，默认是8080

2. 是否开启动态路由
   
   开启动态路由，动态路由也就是符合REST规范的请求，所以不需要额外配置
   
3. 数据文件根目录
   
   本项目构造模拟数据的方式是使用磁盘文件，每个请求会对应磁盘上的一个具体的文件，默认就是这里配置的根目录下URL指向的文件

4. consul
   
   本项目支持consul注册，方便使用微服务的项目使用，比如openfeign


**路由和数据文件**

1. 静态路由
   
   静态路由可以根据需要违反REST规范，或者追加一些额外配置，包括：

   - path: 路由的路径，如/topics
   - method: 指定这条配置在路由匹配时使用的方法，默认是GET
   - action: 可选值是，r(读)/w(覆盖写)/a(追加写)/d(删除)，如果不配置，会随着method的不同而变化，比如GET默认是r，POST/PUT默认是a，DELETE是d
   - single: 是否单一文件模式，默认是false，如果有一个静态路由的action配置成了w，那么要读取写入的数据需要额外再配置一个静态路由，并将single配置成true
   - file: 手动指定url对应的文件
   - fields: 限制返回的属性，默认是不限制
   - unique_not_list：如果结果只有一条数据，那么不使用数组类型的结果
   

2. 动态路由
   
   不需要配置的路由，随着请求的到来会自动去配置的根目录寻找对应的文件，根据Content-Type为请求url追加扩展名，如application/json就追加 .json，这个规则对静态路由也生效
   
3. 参数
   
   查询类的请求支持参数，包括查询字符串和路径变量，如果要对某个请求开启路径变量(如/topics/name/{name})，需要配置一个静态路由(参数的部分不需要配置)，其他属性都不需要配置
   
   如果配置了参数，那么会对文件中的数据按照参数去匹配，只会返回匹配到的结果

4. 数据文件
   
   以json为例，数据文件主要包括以下几部分：

  ``` json
  {
    "post_response": {},
    "del_response": {},
    "datum": {},
    "data": [],
  }
  ```

  - post_response用于个性化action是w/a时的返回值，默认是{"success": true}
  - del_response用于个性化action是d时的返回值，默认是{"success": true}
  - datum用于保存action是w时的传入数据
  - data用于保存action是a时的传入数据

### TODO tcp server
### TODO udp server
### TODO tcp client
### TODO udp client
### TODO mqtt client
