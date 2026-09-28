# noticerender — 本地化邮件/短信通知模板工具

纯 Go 标准库实现的离线模板编译器与批量渲染器。模板目录是 JSON，编译后生成
独立的 JSON 模板包；渲染时读取模板包与 JSONL 请求，逐条输出 JSONL 结果和一个
汇总 JSON，全程不依赖任何在线服务。

## 构建与命令

```sh
go build ./cmd/noticerender

# 编译：catalog.json -> package.json
noticerender compile examples/catalog.json examples/package.json

# 渲染：package.json + requests.jsonl -> results.jsonl + summary.json
noticerender render examples/package.json examples/requests.jsonl \
    examples/results.jsonl examples/summary.json
```

路径传 `-` 时：请求文件从 stdin 读取，结果或汇总写到 stdout。

## 目录（catalog）格式

```json
{
  "default_language": "en",
  "languages": {
    "zh-HK": "zh-CN",
    "zh-CN": "zh",
    "zh": "",
    "fr": "en"
  },
  "templates": {
    "welcome_email": {
      "en": "Hello {name}, welcome to {service}!",
      "zh-CN": "{name}，欢迎使用{service}！",
      "fr": "Bonjour {name}, bienvenue sur {service} !"
    }
  }
}
```

- `default_language`：默认语言，每个模板都必须有该语言文案。
- `languages`：非默认语言到回退语言的映射；值为空（或省略）表示回退到默认
  语言。每种非默认语言至多指定一个回退语言，默认语言不能声明回退。
- `templates`：模板编号 -> 语言 -> 文案。同一编号、同一语言不得重复登记
  （JSON 重复键会直接报错）。

### 编译校验规则

编译会先校验整个目录，任何一条不满足都失败，且**不会生成或覆盖**模板包
（写入采用临时文件 + 原子改名）：

- 每个模板必须有默认语言文案；
- 文案中的语言、回退目标都必须是已声明的已知语言；
- 回退关系中不允许出现环（如 `zh -> zh-CN -> zh`）；
- 所有已有翻译的占位符集合必须与默认文案完全一致（顺序、重复次数不限）；
- 所有文案都必须通过下面的括号语法解析。

## 文案与占位符语法

文案从左到右解析：

- 普通文本中的 `{{` 输出一个字面 `{`，`}}` 输出一个字面 `}`；
  例如 `"{{name}}"` 渲染为 `{name}`，而不是占位符；
- 单个 `{` 开始占位符，名称以英文字母开头，之后只能是英文字母、数字或
  下划线，遇到名称后的第一个 `}` 结束：`{name}`、`{user_2}`；
- 占位符可以在同一文案中重复出现；
- 其余不配对括号（孤立 `}`、`{1x}`、`{na-me}`、未闭合的 `{name` 等）都是
  非法语法。

## 回退解析

渲染时从目标语言开始，沿回退链逐级查找，**在第一个有该模板文案的语言命中
并停止**，不会拼接不同语言的文案。例如 `zh-HK -> zh-CN -> zh -> en`：

- 目标语言有文案：直接使用，不计为回退；
- 在中间或默认语言命中：输出实际采用语言，`fallback: true`；
- 未知模板编号或未知目标语言：该请求明确报错。

## 请求 / 结果 / 汇总

请求（JSONL，每行一个）：

```json
{"template":"welcome_email","language":"zh-HK","params":{"name":"张三","service":"云相册"}}
```

结果（JSONL，按请求行顺序输出，带原始行号）：

```json
{"line":1,"template":"welcome_email","language":"zh-HK","output":"……",
 "used_language":"zh-CN","path":["zh-HK","zh-CN","zh","en"],"fallback":true}
```

失败行只包含 `line` 和具体 `error`，渲染继续处理后续请求：

```json
{"line":8,"error":"missing required param \"minutes\""}
```

参数规则：

- 占位符对应的参数**缺失、多余或不是字符串**（数字、布尔、null、对象、
  数组）都拒绝该请求；
- 参数值原样插入，其中的花括号**不会**被再次解释为模板；
- 空请求文件产生空结果文件和全零汇总；空行被跳过但仍占用行号。

汇总 JSON：

```json
{ "total": 12, "succeeded": 7, "failed": 5, "fallbacks": 2 }
```

`fallbacks` 统计发生过语言回退的**成功**请求数。

## 编译包独立性

编译包保存的是解析后的分段（字面量段 + 占位符段）与解析好的回退关系，渲染
时不再读取原目录。之后修改或删除 catalog 都不会影响已有包的渲染结果；编译
失败时旧包原样保留。

## 示例与测试

- `examples/catalog.json`：含多级回退、仅默认语言模板、括号转义的目录；
- `examples/requests.jsonl` / `examples/results.jsonl` /
  `examples/summary.json`：混合成功与各类失败的批量样例；

运行全部测试：

```sh
go test ./...
```

测试覆盖：括号转义与非法括号、占位符集合冲突、未知语言与回退环、多级回退
命中、缺失/多余/非字符串参数、参数花括号不被二次解析、混合成功失败与行号、
空文件、以及编译失败保留旧包。
