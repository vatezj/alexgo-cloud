"""操作手册章节数据（spec §7.1）。图题编号由渲染时按出场顺序生成。"""

CHAPTERS: list[dict] = [
    {
        "title": "第一章 软件概述",
        "sections": [
            {
                "title": "1.1 软件定位",
                "paragraphs": [
                    "alexGo-cloud 后台管理系统软件是一套基于模块化单体（Modular Monolith）"
                    "架构的后台管理平台，提供系统管理、权限控制、多租户、内容发布与业务数据"
                    "管理等能力。服务端采用 Go 语言开发，管理端为 Web 单页应用，"
                    "二者通过统一的 REST API 交互。",
                ],
                "steps": [],
                "figures": [],
            },
            {
                "title": "1.2 功能清单",
                "paragraphs": ["本软件进册范围内的功能模块如下："],
                "table": {
                    "headers": ["模块", "主要功能"],
                    "rows": [
                        ["系统管理", "用户、角色、菜单、部门、岗位、字典、系统参数、通知公告、租户"],
                        ["日志与安全", "登录日志、操作日志、在线用户、OAuth2 客户端、渠道配置"],
                        ["订单管理", "订单列表查询与维护"],
                        ["基础设施", "代码生成配置"],
                        ["通用能力", "登录认证、全局搜索、消息通知、个人偏好"],
                    ],
                },
                "steps": [],
                "figures": [
                    {"file": "02-dashboard.png", "caption": "系统工作台首页"},
                ],
            },
        ],
    },
    {
        "title": "第二章 运行环境",
        "sections": [
            {
                "title": "2.1 软硬件要求",
                "paragraphs": [
                    "服务端：Go 1.25 及以上、MySQL 8.x、Node.js 22.18 及以上（构建管理端）；"
                    "Redis 与 NATS 为可选组件，默认配置关闭，不启动也可运行。",
                    "管理端：现代浏览器（Chrome / Edge / Safari 最新版）；"
                    "硬件无特殊要求，普通 1GHz 以上 CPU、2GB 内存即可流畅使用。",
                ],
                "steps": [],
                "figures": [],
            },
        ],
    },
    {
        "title": "第三章 安装与启动",
        "sections": [
            {
                "title": "3.1 服务端启动",
                "paragraphs": ["在仓库根目录执行以下命令："],
                "steps": [
                    "创建数据库：mysql -uroot -p -e \"CREATE DATABASE IF NOT EXISTS alexgo DEFAULT CHARSET utf8mb4;\"",
                    "执行数据库迁移：make migrate",
                    "启动服务（默认监听配置端口）：make run",
                    "验证存活：curl http://localhost:<端口>/health，返回 {\"status\":\"ok\"} 即启动成功",
                ],
                "figures": [],
            },
            {
                "title": "3.2 管理端启动",
                "paragraphs": ["服务端就绪后，启动管理端开发服务器："],
                "steps": [
                    "进入前端目录：cd frontend/admin",
                    "安装依赖：pnpm install",
                    "启动：pnpm --filter @vben/web-antd dev",
                    "浏览器访问 http://localhost:5666，默认账号 admin / admin123",
                ],
                "figures": [],
            },
        ],
    },
    {
        "title": "第四章 通用功能",
        "sections": [
            {
                "title": "4.1 登录",
                "paragraphs": ["进入登录页，输入账号密码，完成滑块验证后点击登录。"],
                "steps": ["输入用户名与密码", "按住滑块拖动至最右完成验证", "点击「登录」进入系统"],
                "figures": [{"file": "01-login.png", "caption": "登录页"}],
            },
            {
                "title": "4.2 界面布局",
                "paragraphs": [
                    "左侧为功能导航菜单，顶部为标签页与快捷工具区，中部为内容区。"
                    "工作台首页展示待办、项目与快捷导航。",
                ],
                "steps": [],
                "figures": [{"file": "02-dashboard.png", "caption": "工作台与整体布局"}],
            },
            {
                "title": "4.3 全局搜索",
                "paragraphs": ["点击顶部「搜索」或按 ⌘K / Ctrl+K 唤起全局搜索面板，可快速跳转页面。"],
                "steps": ["点击顶部搜索框或按快捷键", "输入关键字", "回车或点击结果跳转"],
                "figures": [{"file": "22-search.png", "caption": "全局搜索面板"}],
            },
            {
                "title": "4.4 消息通知",
                "paragraphs": ["点击右上角铃铛查看站内通知；「我的通知」页面支持已读标记与查看全部。"],
                "steps": ["展开「系统管理 → 我的通知」", "查看通知列表与详情"],
                "figures": [{"file": "17-my-notices.png", "caption": "我的通知列表"}],
            },
            {
                "title": "4.5 用户菜单",
                "paragraphs": ["点击右上角头像展开用户菜单，可进入锁定屏幕或退出登录。"],
                "steps": ["点击头像展开菜单", "点击「退出登录」返回登录页"],
                "figures": [{"file": "23-logout.png", "caption": "用户菜单与退出登录入口"}],
            },
        ],
    },
    {
        "title": "第五章 功能模块操作",
        "sections": [
            {
                "title": "5.1 用户管理",
                "paragraphs": ["管理后台登录账号，支持新增、编辑、启停用与分配角色。"],
                "steps": ["展开「系统管理 → 用户管理」", "点击「新增」填写用户信息并分配角色", "列表中可编辑或启停用"],
                "figures": [{"file": "03-users.png", "caption": "用户列表"}],
            },
            {
                "title": "5.2 角色管理",
                "paragraphs": ["定义角色并为其勾选菜单权限，控制可见页面与操作按钮。"],
                "steps": ["展开「系统管理 → 角色管理」", "新增角色并勾选数据权限与菜单权限", "保存后对用户生效"],
                "figures": [{"file": "04-roles.png", "caption": "角色列表"}],
            },
            {
                "title": "5.3 菜单管理",
                "paragraphs": ["维护左侧导航的目录、页面与按钮级权限标识。"],
                "steps": ["展开「系统管理 → 菜单管理」", "在目录节点下新增页面或按钮", "配置权限标识与排序"],
                "figures": [{"file": "05-menus.png", "caption": "菜单树"}],
            },
            {
                "title": "5.4 部门与岗位",
                "paragraphs": ["部门树用于组织架构与数据权限；岗位用于人员任职标注。"],
                "steps": ["展开「部门管理」维护组织树", "展开「岗位管理」维护岗位字典"],
                "figures": [
                    {"file": "06-depts.png", "caption": "部门树"},
                    {"file": "07-posts.png", "caption": "岗位列表"},
                ],
            },
            {
                "title": "5.5 字典管理",
                "paragraphs": ["维护系统下拉选项的类型与明细，供各表单复用。"],
                "steps": ["展开「系统管理 → 字典管理」", "新增字典类型", "在类型下维护明细项"],
                "figures": [{"file": "08-dict.png", "caption": "字典类型与明细"}],
            },
            {
                "title": "5.6 系统参数",
                "paragraphs": ["维护全局配置项（键值对），供业务代码读取。"],
                "steps": ["展开「系统管理 → 系统参数」", "新增或编辑配置键值"],
                "figures": [{"file": "09-configs.png", "caption": "系统参数列表"}],
            },
            {
                "title": "5.7 通知公告",
                "paragraphs": [
                    "发布面向全员的公告，支持富文本内容与附件上传（文件上传能力入口）。",
                ],
                "steps": [
                    "展开「系统管理 → 通知公告」",
                    "点击「新建公告」，填写标题与内容",
                    "点击「点击上传」添加附件后保存",
                ],
                "figures": [
                    {"file": "10-notices.png", "caption": "公告列表"},
                    {"file": "11-notice-upload.png", "caption": "公告编辑器与附件上传"},
                    {"file": "24-notice-detail.png", "caption": "公告内容编辑"},
                ],
            },
            {
                "title": "5.8 租户管理",
                "paragraphs": ["多租户模式下维护租户主体、状态与配额。"],
                "steps": ["展开「系统管理 → 租户管理」", "新增租户并配置状态"],
                "figures": [{"file": "12-tenants.png", "caption": "租户列表"}],
            },
            {
                "title": "5.9 日志审计",
                "paragraphs": ["登录日志记录认证行为；操作日志记录后台写操作，用于安全审计。"],
                "steps": ["展开「登录日志」查看认证记录", "展开「操作日志」查看写操作明细"],
                "figures": [
                    {"file": "13-login-logs.png", "caption": "登录日志"},
                    {"file": "14-operate-logs.png", "caption": "操作日志"},
                ],
            },
            {
                "title": "5.10 在线用户与会员查询",
                "paragraphs": [
                    "在线用户页展示当前活跃会话，支持踢出与代登录；"
                    "代登录弹窗内嵌会员按昵称/手机过滤查询（会员管理能力入口）。",
                ],
                "steps": [
                    "展开「系统管理 → 在线用户」查看活跃会话",
                    "点击行内「踢出」强制下线",
                    "点击行内「代登录」，在弹窗中按昵称/手机过滤会员",
                ],
                "figures": [
                    {"file": "15-online-users.png", "caption": "在线用户列表"},
                    {"file": "16-member-search.png", "caption": "代登录弹窗与会员过滤"},
                ],
            },
            {
                "title": "5.11 OAuth2 客户端与渠道配置",
                "paragraphs": ["管理 SSO 授权码客户端与第三方登录渠道。"],
                "steps": ["展开「OAuth2 → 客户端管理」维护客户端", "展开「渠道配置」维护社交登录渠道"],
                "figures": [
                    {"file": "18-oauth2-clients.png", "caption": "OAuth2 客户端列表"},
                    {"file": "19-social-apps.png", "caption": "渠道配置列表"},
                ],
            },
            {
                "title": "5.12 订单管理",
                "paragraphs": ["订单业务数据的列表查询与维护示例。"],
                "steps": ["展开「订单 → 订单列表」", "按条件查询并维护订单"],
                "figures": [{"file": "20-order.png", "caption": "订单列表"}],
            },
            {
                "title": "5.13 代码生成配置",
                "paragraphs": ["基于数据表配置在线生成 CRUD 代码（配置表、预览、下载）。"],
                "steps": ["展开「基础设施 → 代码生成配置」", "维护表配置并预览生成结果"],
                "figures": [{"file": "21-codegen.png", "caption": "代码生成配置表"}],
            },
        ],
    },
    {
        "title": "第六章 常见问题",
        "sections": [
            {
                "title": "6.1 故障处理",
                "paragraphs": [],
                "steps": [
                    "端口被占用：更换监听端口或结束占用进程后重启服务",
                    "数据库连接失败：确认 MySQL 已启动、DSN 配置正确并已执行 make migrate",
                    "管理端登录 401：确认服务端已启动且登录接口代理配置正确",
                    "左侧菜单不显示：确认当前角色已勾选对应菜单权限（菜单管理 → 角色授权）",
                ],
                "figures": [],
            },
        ],
    },
]
