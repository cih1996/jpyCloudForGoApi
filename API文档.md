
## 使用说明

### 服务器地址
> 程序运行后会监听2个端口，1001端口为http/wss接口，1002端口为ws接口。

网页示例：http://127.0.0.1:1001

> 以下数据主要使用该ws连接

接口连接地址：
ws://127.0.0.1:1001/api/unified/ws


### 登录
```json
{
  "type": "Login",
  "seq": 5027,
  "data": null,
  "token": "10002ac0e042f7331325739102ef2c489d5ae1770013344585"
}
```

```json
{
  "type": "Login",
  "code": 200,
  "msg": "Success",
  "seq": 5027
}
```

---

### 获取设备列表
```json
{"type": "GetDeviceList", "seq": 6635, "data": null}
```


```json
{"type": "GetDeviceList", "code": 200, "msg": "Success", "data": {"records": [{"tbYunJiUserDeviceId": 10001, "deviceId": 112150, "createdAt": 1769859913216, "userId": 10002, "expiresTime": 1801395913213, "buyType": 2, "yunjiUserGroupId": 0, "remark": "", "deviceInfo": {"deviceId": 112150, "typ": 1, "uuid": "04111JEC200444", "version": "12", "brand": "YFYA4", "info": "{\"seat\":22,\"uuid\":\"04111JEC200444\",\"model\":\"YFYA4\",\"osVersion\":\"YFYA4_android_12_20260130_v1.0\",\"androidVersion\":\"12\",\"brand\":\"\",\"width\":1080,\"height\":2340,\"online\":0,\"ip\":\"\",\"type\":0}", "createdAt": 1769778901393, "updatedAt": 1770029153, "onlineTime": 0, "online": false, "ip": "192.168.10.122", "status": 2, "deptId": 10007, "buyUserId": 10002, "tbShopId": 10001, "tbProxyId": 438, "s5info": ""}}], "total": 24, "page": 1, "pageSize": 999999, "totalPages": 1}, "seq": 6635}
```

--- 
### 设置S5代理

```json
{"type": "setSocket5", "seq": 5450, "data": {"deviceId": 112132, "s5Url": "socks5://cyygameeip:CyGame@114.67.224.46:44016", "nOutSwID": 10006}}
```

```json
{
  "type": "setSocket5",
  "code": 200,
  "msg": "Success",
  "seq": 5450
}
```


--- 

### 获取S5出口路线

```json
{"type": "getS5outLine", "seq": 5966, "data": {"deviceId": 112132}}
```

```json
{
  "type": "getS5outLine",
  "code": 200,
  "msg": "Success",
  "data": [
    {
      "nSwID": 10006,
      "szName": "PPPOE",
      "szAlias": "",
      "nVrfID": 0,
      "nPosition": 0
    }
  ],
  "seq": 2698
}
```

---

### 改机

```json
[{"deviceId": 112132, "category": "491", "bs": "wifi", "operator": "00", "timezone": "America/New_York", "language": "en-US", "version": "491", "country": "us", "operatorName": "AmeriLink", "mcc": "310", "mnc": "630", "msisdn": "", "smsc": ""}]
```

```json
{
  "type": "Changephones",
  "code": 200,
  "msg": "Success",
  "data": [
    {
      "deptId": 10007,
      "deviceId": 112132,
      "id": 13118,
    }
  ],
  "seq": 5765
}
```
---

### 查询改机状态

```json
{"type": "getTaskStatus", "seq": 2746, "data": {"tbChangeOsIds": [13118]}}
```

```json
{
  "type": "getTaskStatus",
  "code": 200,
  "msg": "Success",
  "data": [
    {
      "addTime": 1770033096
      "deptId": 10007,
      "deviceId": 112132,
      "id": 13118,
      "progress": "改机完成",
      "reqJson": "{\"deviceId\":112132,\"category\":\"491\",\"bs\":\"wifi\",\"operator\":\"00\",\"timezone\":\"America/New_York\",\"language\":\"en-US\",\"version\":\"491\",\"country\":\"us\",\"operatorName\":\"AmeriLink\",\"mcc\":\"310\",\"mnc\":\"630\",\"msisdn\":\"\",\"smsc\":\"\"}",
      "sn": "11031JEC201036",
      "status": 200,
      "userId": 10002
    }
  ],
  "seq": 2746
}
```

---

### 获取文件列表

```json
{"type": "getUserFiles", "seq": 7637, "data": {"fileName": ""}}
```

```json
{
  "type": "getUserFiles",
  "code": 200,
  "msg": "Success",
  "data": [
    {
      "addTime": 1769859949638,
      "delTime": 0,
      "fileId": 10017,
      "fileName": "检测环境安全.apk",
      "hash": "27cef6c9e720a1279fb7a360aab780938501a60941a9a783704bf1fe1df7128c",
      "size": 13893050,
      "url": "https://ad-resource.s3.cn-north-1.jdcloud-oss.com/uploadFileTest/27cef6c9e720a1279fb7a360aab780938501a60941a9a783704bf1fe1df7128c",
      "userId": 10002
    },
    {
      "addTime": 1769860085059,
      "delTime": 0,
      "fileId": 10018,
      "fileName": "应用宝.apk",
      "hash": "9e9e452292f5b0335554e0e161035a0738dc048ccf28f0d0dea1b169a19d7999",
      "size": 32046264,
      "url": "https://ad-resource.s3.cn-north-1.jdcloud-oss.com/uploadFileTest/9e9e452292f5b0335554e0e161035a0738dc048ccf28f0d0dea1b169a19d7999",
      "userId": 10002
    }
  ],
  "seq": 1716
}
```

---

### 下载文件任务推送

```json
{"type": "downLoadInstallApp", "seq": 7022, "data": {"devices": [112133], "url": "https://ad-resource.s3.cn-north-1.jdcloud-oss.com/uploadFileTest/27cef6c9e720a1279fb7a360aab780938501a60941a9a783704bf1fe1df7128c", "install": true}}
```


```json
{
  "type": "downLoadInstallApp",
  "code": 200,
  "msg": "Success",
  "data": {
    "data": {
      "beginTime": 1770033843,
      "id": 1,
      "install": true,
      "lastTime": 0,
      "md5": null,
      "path": null,
      "process": 0,
      "status": null,
      "url": "https://ad-resource.s3.cn-north-1.jdcloud-oss.com/uploadFileTest/27cef6c9e720a1279fb7a360aab780938501a60941a9a783704bf1fe1df7128c"
    },
    "f": 293,
    "msg": "操作成功",
    "req": false,
    "seq": 1
  },
  "seq": 7022
}
```

---


### 下载文件进度查询

```json
{"type": "getDownloadProgress", "seq": 487, "data": {"deviceId": 112133, "id": "1"}}
```


```json
{
  "type": "getDownloadProgress",
  "code": 200,
  "msg": "Success",
  "data": {
    "data": {
      "beginTime": 1770033843,
      "id": 1,
      "install": true,
      "lastTime": 1770033844,
      "md5": null,
      "path": "/sdcard/download_1/null",
      "process": 1,
      "status": 4,
      "url": "https://ad-resource.s3.cn-north-1.jdcloud-oss.com/uploadFileTest/27cef6c9e720a1279fb7a360aab780938501a60941a9a783704bf1fe1df7128c"
    },
    "f": 294,
    "msg": "操作成功",
    "req": false,
    "seq": 3
  },
  "seq": 487
}
```


## 获取应用列表

```json
{"type": "getAppList", "seq": 4990, "data": {"deviceId": 112133}}
```


```json
{
  "type": "getAppList",
  "code": 200,
  "msg": "Success",
  "data": {
    "data": [
      {
        "appname": "应用宝",
        "firstInstallTime": 1770034186460,
        "lastUpdateTime": 1770034186460,
        "packageName": "com.tencent.android.qqdownloader"
      }
    ],
    "f": 290,
    "msg": "操作成功",
    "req": false,
    "seq": 1
  },
  "seq": 4990
}
```


## 启动应用

```json
{"type": "startApp", "seq": 188, "data": {"deviceId": 112133, "packageName": "com.tencent.android.qqdownloader"}}
```


```json
{
  "type": "startApp",
  "code": 200,
  "msg": "Success",
  "data": {
    "data": true,
    "f": 291,
    "msg": "操作成功",
    "req": false,
    "seq": 3
  },
  "seq": 188
}
```


### 隐藏App

```json
{
  "type": "hideApp",
  "seq": 6633,
  "data": {
    "deviceId": 112132,
    "packageName": "com.google.android.keep",
    "isHide": true
  }
```

```json
{
  "type": "hideApp",
  "code": 200,
  "msg": "Success",
  "data": "Success",
  "seq": 6633
}
```

### 设置定位

```json
{
  "type": "setLocation",
  "seq": 7742,
  "data": [
    {
      "deviceId": 112132,
      "lat": 0,
      "lng": 0
    }
  ]
}
```

```json
 {
  "type": "setLocation",
  "code": 200,
  "msg": "Success",
  "data": [
    {
      "deviceId": 79618,
      "result": "success"
    }
  ],
  "seq": 8513
}
```

---

### 执行shell命令
```json
{
  "type": "execShell",
  "seq": 9400,
  "data": {
    "deviceId": 112132,
    "shell": "ls -lh /sdcard/"
  }
}
```
```json
{
  "type": "execShell",
  "code": 200,
  "msg": "Success",
  "data": {
    "data": "total 48K\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 Alarms\ndrwxrwx--- 5 root everybody 3.4K 2026-02-02 06:52 Android\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 Audiobooks\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 DCIM\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 Documents\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 Download\ndrwxrwx--- 3 root everybody 3.4K 2026-02-02 06:52 Movies\ndrwxrwx--- 3 root everybody 3.4K 2026-02-02 06:52 Music\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 Notifications\ndrwxrwx--- 3 root everybody 3.4K 2026-02-02 06:52 Pictures\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 Podcasts\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 Recordings\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 Ringtones\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 07:19 download_1\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 nmJAKP\ndrwxrwx--- 2 root everybody 3.4K 2026-02-02 06:52 reports",
    "f": 289,
    "msg": "操作成功",
    "req": false,
    "seq": 5
  },
  "seq": 9400
}
```