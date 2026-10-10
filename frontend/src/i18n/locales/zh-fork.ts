/**
 * zh fork 差异字典（生成文件，可手动维护）。
 *
 * - 仅包含本 fork 独有的 key，以及与上游 locales/zh/ 不同的文案；
 *   合并时本文件的值优先（见 locales/zh.ts 的 deepMerge）。
 * - 修改 fork 文案：直接编辑本文件。
 * - 同步上游新文案：更新 locales/zh/ 下的模块化文件。
 * - 重新生成本文件：node scripts/gen-fork-locales.mjs
 *   （注意：会以当前 locales/zh.ts 为基准覆盖本文件，手改内容会丢失，
 *    只有在明确要重新提取差异时才运行。）
 */
export default {
  home: {
    viewOnGithub: 'GitHub',
    viewDocs: '文档',
    enterprise: '企业服务',
    models: '模型广场',
    getStarted: '领取 $10 ≈ 330M 免费 Tokens',
    heroDescription: '用一套 API 接入 DeepSeek、Qwen、Kimi、GLM 等开源大模型，价格仅为 GPT-5.5 的 1/33',
    tags: {
      subscriptionToApi: '兼容 OpenAI',
      stickySession: '美国节点部署'
    },
    hero: {
      slogan: 'DeepSeek、Qwen、Kimi、GLM — 最低价格仅为GPT的 1/33',
      title: 'Three Router',
      subtitle: '顶尖、超高性价比的开源大模型，企业级LLM服务。',
      cta: '领取 $10 ≈ 330M 免费 Tokens →',
      codeHint: '只需改一行 URL，其他代码不变。',
      coreDiff: '美国本地部署 + 最高降低 97% 成本',
      priceAdvantage: '只需 1/33 的价格。从不按原价付费。',
      priceReasons: '美东 & 美西节点 · 云厂商直供',
      tags: {
        discount: 'DeepSeek V4-Pro 仅 $0.42/M',
        deployment: '美国本地部署',
        freeTokens: '新用户获赠 $10 ≈ 330M 免费 Tokens'
      },
      costChart: {
        eyebrow: '开源与闭源模型成本对比',
        title: '同样 $5 预算，开源旗舰模型可跑更多请求',
        usRoute: '部署位置',
        maxSaving: '最高节省',
        note: '以 $/M 输入 tokens 对比：美国本地入口保持低延迟，开源模型成本曲线显著下探。'
      },
      pricingSection: {
        tableHeader: {
          title: '价格对比',
          model: '模型',
          capability: 'threerouter ($/M 输入)',
          price: 'OpenAI ($/M 输入)',
          relative: '节省比例'
        },
        models: {
          gpt55: {
            name: 'GPT-5.5',
            capability: 'OpenAI 最新旗舰'
          },
          claudeOpus48: {
            name: 'Claude Opus 4.8',
            capability: 'Anthropic 最新旗舰，SWE-bench Pro 69.2%'
          },
          glm51: {
            name: 'GLM-5.2',
            capability: '智谱旗舰，SWE-bench第一梯队'
          },
          kimiK26: {
            name: 'Kimi K2.6',
            capability: '开源SOTA，SWE-Bench Pro 58.6分'
          },
          deepseekV4Flash: {
            name: 'DeepSeek V4-Flash',
            capability: '性价比之选，MoE 架构'
          },
          deepseekV4Pro: {
            name: 'DeepSeek V4-Pro',
            capability: '旗舰推理，Codeforces #1'
          },
          minimaxM3: {
            name: 'MiniMax-M3',
            capability: '1M上下文，前沿编码与原生多模态'
          },
          qwen37Max: {
            name: 'Qwen3.8-Max',
            capability: '阿里智能体旗舰，1M上下文，顶级编码，复杂工作流与多框架泛化'
          },
          seedance20: {
            name: 'Seedance-2.0',
            capability: '字节视频生成，电影级多模态输出'
          }
        },
        baseline: '基准价',
        discount: '节省 {percent}%',
        directProcurement: '云厂商与AI平台直采',
        directProcurementDesc: '直接从 AWS、GCP、Azure、阿里云等官方云厂商采购。非转售，非灰色通道。提供安全团队可审计的完整溯源链路。',
        enterpriseBadge: '企业合规网关',
        benefits: {
          title: '企业级优势',
          compliant: '符合行业合规要求',
          stable: '99.99% 服务可用性',
          traceable: '完整审计追踪与认证',
          secure: '企业级安全保障'
        }
      }
    },
    painPoints: {
      items: {
        expensive: {
          title: 'GPT 贵 36 倍',
          desc: 'GPT-5.5 要 $5/M，DeepSeek V4-Pro 只要 $0.42/M。同样的智能，1/12 的价格。'
        },
        complex: {
          title: 'API Key 管理混乱',
          desc: '要在不同平台分别管理 DeepSeek、Qwen、Kimi、GLM 的账号和密钥'
        },
        unstable: {
          title: '远距离直连延迟高',
          desc: '大多数开源大模型 API 走远距离路由。我们走美东和美西节点，延迟低于 200ms。'
        },
        noControl: {
          title: '用量无法追踪',
          desc: '不知道每个模型、每个团队、每个 API Key 花了多少 tokens'
        }
      }
    },
    solutions: {
      title: '一个 API，接入所有开源大模型。',
      subtitle: '三步，大幅降低你的 LLM 开销'
    },
    features: {
      unifiedGateway: '一个 API 密钥',
      unifiedGatewayDesc: '通过一个 API 密钥调用 DeepSeek、Qwen、Kimi、GLM、MiniMax、Seedance。完全兼容 OpenAI 格式。',
      multiAccount: '美国本地路由',
      multiAccountDesc: '请求走美东和美西节点，P99 延迟低于 200ms——即使模型在上海。',
      balanceQuotaDesc: '按实际用量计费，支持配额上限。每个模型、每个团队的 token 消耗一目了然。'
    },
    easyrouterAdvantages: {
      eyebrow: '为什么选 threerouter',
      title: '为什么选 threerouter，而不是其他网关？',
      subtitle: '美国本地基础设施。云厂商直采。OpenAI 即插即用。',
      ultraFast: {
        title: '美国本地节点，而非远距离节点',
        desc: '大多数开源大模型 API 走远距离路由。我们不。你的请求走美东/美西节点，P99 延迟低于 200ms——即使模型部署在远距离节点。'
      },
      reliable: {
        title: '合规性可展示给法务',
        desc: '直接从 AWS、GCP、Azure、阿里云采购。非转售，非灰色通道。提供安全团队可审计的完整溯源链路。'
      },
      standardApi: {
        title: '兼容 OpenAI，无需重写代码',
        desc: '即插即用。现有代码、现有工具、现有工作流不变。改一个 URL，其他一切照旧。'
      },
      cheap: {
        title: '超级便宜',
        desc: '价格只有旗舰供应商的 1/33，最高可为您降低 97% 的 AI 成本'
      }
    },
    easyrouterFaq: {
      eyebrow: 'FAQ',
      title: '常见问题解答',
      subtitle: '关于服务、计费和集成，你想知道的都在这里。',
      tabs: {
        service: '关于服务',
        billing: '价格与计费',
        integration: '集成与使用'
      },
      service: {
        q1: '你们是代理还是转售商？',
        a1: '都不是。我们是企业级 AI API 网关。上游服务来自 AWS、GCP、Azure 等官方顶级云厂商；对于开源模型，直接来自模型厂商或全球认可的推理服务。链路合法、透明、可审计。',
        q2: '你们的模型资源来自哪里？',
        a2: '资源来自官方云厂商、AI 平台、模型厂商以及全球认可的推理服务，确保访问稳定、采购合规、交付链路可溯源。',
        q3: '你们会存储用户请求数据吗？',
        a3: '不会。网关以安全透传和路由为核心设计，不会将客户提示词或响应内容用于模型训练，并尽量减少运营安全所需之外的数据留存。',
        q4: '支持哪些模型？',
        a4: '覆盖 DeepSeek、Qwen、Kimi、MiniMax、GLM 以及 Seedance 视频生成模型。更多模型持续接入中。'
      },
      billing: {
        q1: '这是包月订阅吗？',
        a1: '不是。额度按实际模型用量扣减，你只为真实消耗付费，无需为未使用的固定月费买单。',
        q2: '用量如何计费？',
        a2: '文本模型按 token 用量计费；图片、视频、语音等模型按对应供应商的计费单位扣费（图片张数、视频秒数、音频时长）。',
        q3: '价格是否透明？',
        a3: '透明。价格按模型和计费单位展示，使用前可预估成本，使用后可在后台查看实际消耗。',
        q4: '团队可以共享额度吗？',
        a4: '可以。团队和企业可通过共享额度池、权限控制和用量分析统一管理。'
      },
      integration: {
        q1: '是否兼容 OpenAI API？',
        a1: '兼容。完全兼容 OpenAI API 格式，替换 base URL 和 API key 即可开始调用。',
        q2: '迁移成本高吗？',
        a2: '只需一行代码。SDK、Agent 和工作流工具继续使用熟悉的请求格式——只需指向 threerouter.com/v1。',
        q3: '支持故障转移和智能路由吗？',
        a3: '支持。通过多供应商路由、负载均衡和故障转移策略提升可用性。',
        q4: '可以用于生产环境吗？',
        a4: '可以。网关面向生产场景设计，提供标准化 API、高可用路由、用量可观测能力以及企业合规实践。'
      }
    },
    comparison: {
      headers: {
        official: '直接 API 订阅',
        us: 'threerouter'
      }
    },
    providers: {
      loginToView: '登录查看'
    },
    cta: {
      title: '别再为同样的智能多付 33 倍。',
      description: '领取 $10 ≈ 330M 免费 tokens，无需信用卡。',
      button: '领取 $10 ≈ 330M 免费 Tokens — 无需信用卡 →'
    },
    reviews: {
      title: '为开发者和企业打造',
      subtitle: '生产环境真实数据',
      review1: {
        text: '过去 24 小时 API 调用',
        name: '120 万+',
        role: '请求已处理'
      },
      review2: {
        text: '美东节点 P99 延迟',
        name: '187ms',
        role: '平均响应时间'
      },
      review3: {
        text: '过去 90 天可用性',
        name: '99.99%',
        role: '服务可用率'
      },
      review4: {
        text: '客户数据留存',
        name: '零',
        role: '隐私优先设计'
      },
      review5: {
        text: '社区评价',
        name: '1.2K ★',
        role: 'Stars'
      },
      review6: {
        text: '性能排名',
        name: '#1 当日产品',
        role: '排名'
      },
      review7: {
        text: 'G2 用户评分',
        name: '4.8 ★',
        role: '已验证评价'
      },
      review8: {
        text: '可用模型',
        name: '15+',
        role: '持续增长中'
      }
    },
    testimonials: {
      eyebrow: '真实用户反馈',
      title: '开发者怎么说',
      subtitle: '来自生产环境的一线使用体验',
      t1: {
        name: 'Alex Chen',
        role: 'SaaS 创始人',
        quote: '接入 Three Router 后，AI 调用成本直接从每月 $4,000 降到 $120。API 完全兼容，迁移只花了 10 分钟。'
      },
      t2: {
        name: 'Maria Rodriguez',
        role: '全栈工程师',
        quote: 'DeepSeek V4-Pro 通过美国本地节点调用延迟很低，价格却只有 OpenAI 的 1/12。对我们来说这就是生产环境的默认选择。'
      },
      t3: {
        name: 'Sam Liu',
        role: 'AI 创业公司 CTO',
        quote: '把生产环境的 LLM 调用全部切到 Three Router 后，成本下降了 90% 以上，而且稳定性非常出色。'
      }
    },
    compliancePromo: {
      eyebrow: 'AI 治理与合规',
      title: '为企业 AI 应用提供合规保障',
      description: 'ThreeRouter 内置 EU AI Act、GDPR 等全球法规合规框架，自动生成合规凭证与处理记录。',
      button: '了解合规能力',
      badge: '企业级合规',
      features: {
        euai: {
          title: 'EU AI Act 合规评估',
          desc: 'AI系统角色定位与法律映射，基于Annex III的高风险场景评估。'
        },
        gdpr: {
          title: 'GDPR Art.30 ROPA',
          desc: '完整的处理活动记录，合规的合法依据声明，EU SCC跨境传输保障。'
        },
        zdr: {
          title: '零数据保留架构',
          desc: '默认不保留请求内容，灵活的数据保留策略配置，数据最小化原则贯彻。'
        },
        creds: {
          title: '一站式合规凭证',
          desc: '五种合规凭证自动生成，一键导出合规报告，包括GDPR、EU AI Act等。'
        },
        templates: {
          title: '行业合规模板',
          desc: '医疗、金融、教育、电商四大行业模板，预置规则开箱即用。'
        },
        risk: {
          title: '风险分析与监控',
          desc: '实时风险标签监控，异常行为检测，合规政策违规预警，审计日志追溯。'
        }
      },
      certsLabel: '合规认证：'
    },
    footer: {
      documentation: '帮助文档',
      advantage: '我们的优势',
      contact: '联系我们',
      legalNotice: '法律通知请先通过邮箱联系我们'
    }
  },
  common: {
    expandAll: '全部展开',
    collapseAll: '全部收起'
  },
  validation: {
    required: '{field}为必填项',
    maxLength: '{field}长度不能超过{max}个字符',
    minLength: '{field}长度不能少于{min}个字符'
  },
  nav: {
    models: '模型广场',
    teamManagement: '团队管理',
    teamMembers: '团队成员',
    departments: '部门管理',
    consumers: '消费者管理',
    teamAnalytics: '团队统计',
    teamSettings: '团队设置',
    tickets: '我的工单',
    ticketManagement: '工单管理',
    governance: 'AI 治理与合规',
    promptAudit: 'Prompt 审核'
  },
  tickets: {
    fields: {
      id: '编号',
      contact: '联系方式',
      title: '标题',
      category: '分类',
      priority: '优先级',
      status: '状态',
      content: '问题描述',
      updatedAt: '更新时间'
    },
    pricing: {
      input: '输入',
      output: '输出',
      approx: '大约'
    },
    categories: {
      account: '账号问题',
      billing: '余额/计费',
      api: 'API 调用问题',
      model: '模型/渠道问题',
      other: '其他'
    },
    priorities: {
      low: '低',
      normal: '普通',
      high: '高',
      urgent: '紧急'
    },
    statuses: {
      open: '待处理',
      pending: '等待用户回复',
      answered: '已回复',
      closed: '已关闭'
    },
    filters: {
      allStatuses: '全部状态',
      allCategories: '全部分类'
    },
    author: {
      user: '用户',
      admin: '客服'
    },
    placeholders: {
      contact: '邮箱、QQ 或其他联系方式',
      title: '简要描述你的问题',
      content: '请尽量详细描述问题、接口地址、报错信息或相关上下文'
    },
    actions: {
      new: '提交工单',
      submit: '提交工单',
      submitting: '提交中...',
      reply: '发送回复',
      backToList: '返回工单列表'
    },
    new: {
      title: '提交工单',
      description: '遇到账号、计费、API 或模型问题时，可以在这里提交工单。',
      loggedHint: '当前已登录，工单会自动关联到你的账号。',
      guestHint: '你当前未登录，请务必留下有效联系方式。'
    },
    my: {
      title: '我的工单',
      description: '查看你提交的工单状态和客服回复。',
      empty: '暂无工单。'
    },
    detail: {
      title: '工单详情',
      reply: '继续回复',
      closedHint: '该工单已关闭，无法继续回复。'
    },
    messages: {
      created: '工单已提交',
      createFailed: '提交工单失败',
      loadFailed: '加载工单失败',
      replied: '回复已发送',
      replyFailed: '发送回复失败'
    }
  },
  auth: {
    verificationCodeHint: '请输入发送到您邮箱的验证码',
    invalidCode: '请输入有效的验证码'
  },
  keys: {
    departmentLabel: '部门',
    consumerLabel: '消费者（员工）',
    selectDepartment: '选择部门',
    selectConsumer: '选择消费者',
    noDepartment: '无部门',
    noDepartments: '暂无部门',
    noConsumer: '无消费者'
  },
  usage: {
    costDetails: '成本明细',
    imageOutputCost: '图片输出成本'
  },
  admin: {
    models: {
      title: '模型广场',
      description: '管理和配置可用的AI模型',
      copy: '复制模型名称',
      hint: '更多模型通过 API Key 获取',
      loginHint: '登录后查看优惠价格',
      loginAction: '立即登录',
      status: {
        available: '可用'
      },
      pricing: {
        input: '输入',
        output: '输出',
        approx: '大约'
      },
      categories: {
        text: '文本',
        image: '图像',
        audio: '语音',
        multimodal: '多模态'
      }
    },
    team: {
      members: {
        title: '团队成员',
        subtitle: '管理团队成员信息、角色和权限',
        description: '管理团队成员',
        addMember: '添加成员',
        totalMembers: '总成员数',
        activeMembers: '活跃成员',
        admins: '管理员',
        pending: '待处理',
        searchPlaceholder: '搜索成员姓名或邮箱',
        allRoles: '所有角色',
        allStatus: '所有状态',
        status: {
          active: '活跃',
          pending: '待处理',
          inactive: '已禁用'
        },
        columns: {
          name: '姓名',
          role: '角色',
          department: '部门',
          status: '状态',
          joinedAt: '加入时间',
          actions: '操作'
        },
        roles: {
          owner: '所有者',
          admin: '管理员',
          manager: '经理',
          member: '成员',
          viewer: '访客'
        },
        noMembers: '暂无成员',
        addFirstMember: '添加第一位团队成员开始使用',
        deleteMember: '删除成员',
        deleteConfirm: '确定要删除成员 {name} 吗？此操作不可恢复。',
        editNotImplemented: '编辑功能尚未实现',
        memberEnabled: '成员已启用',
        memberDisabled: '成员已禁用',
        memberDeleted: '成员已删除',
        inviteMember: '邀请成员',
        emailLabel: '邮箱',
        emailPlaceholder: '请输入成员邮箱',
        roleLabel: '角色',
        sendInvite: '发送邀请',
        inviting: '发送中...',
        emailRequired: '邮箱为必填项',
        emailInvalid: '邮箱格式不正确',
        editRole: '编辑角色',
        roleDescriptions: {
          owner: '拥有团队完全控制权',
          admin: '管理成员、设置和账单',
          manager: '管理部门和团队运营',
          member: '使用团队资源和查看分析',
          viewer: '仅查看团队数据'
        },
        fetchFailed: '获取团队成员失败',
        inviteSuccess: '邀请成员成功',
        inviteFailed: '邀请成员失败',
        inviteNotImplemented: '邀请功能尚未实现',
        removeMember: '移除成员',
        removeConfirm: '确定要移除成员 {name} 吗？此操作无法撤销。',
        roleUpdated: '成员角色更新成功'
      },
      departments: {
        title: '部门管理',
        subtitle: '管理部门结构和人员配置',
        description: '管理部门结构',
        addDepartment: '添加部门',
        members: '成员数',
        managers: '经理数',
        head: '负责人',
        deleteDepartment: '删除部门',
        deleteConfirm: '确定要删除部门 {name} 吗？此操作不可恢复。',
        editNotImplemented: '编辑功能尚未实现',
        departmentDeleted: '部门已删除',
        name: '部门名称',
        namePlaceholder: '请输入部门名称',
        code: '部门编码',
        codePlaceholder: '请输入部门编码',
        costCenterCode: '部门编码',
        costCenterCodePlaceholder: '请输入部门编码',
        descriptionLabel: '描述',
        descriptionPlaceholder: '请输入部门描述',
        parentDepartment: '上级部门',
        selectParent: '选择上级部门',
        noParent: '无上级（顶级）',
        createDepartment: '创建部门',
        editDepartment: '编辑部门',
        addChild: '添加子部门',
        noDepartments: '暂无部门',
        noResults: '未找到匹配的部门',
        noResultsDescription: '请尝试调整搜索条件或筛选器',
        addFirstDepartment: '创建您的第一个部门以开始',
        fetchFailed: '获取部门列表失败',
        createSuccess: '部门创建成功',
        createFailed: '创建部门失败',
        editSuccess: '部门更新成功',
        updateFailed: '更新部门失败',
        deleteFailed: '删除部门失败',
        searchPlaceholder: '搜索部门名称或描述',
        allStatus: '所有状态',
        createdAt: '创建时间',
        treeView: '树形视图',
        gridView: '网格视图',
        reorderSuccess: '部门层级已更新',
        reorderFailed: '更新部门层级失败',
        reorderCycle: '不能将部门移动到其子部门下',
        status: {
          active: '活跃',
          inactive: '已停用'
        }
      },
      consumers: {
        title: '消费者管理',
        subtitle: '管理消费者账户和使用情况',
        description: '管理消费者',
        addConsumer: '添加消费者',
        totalConsumers: '总消费者数',
        activeConsumers: '活跃消费者',
        newThisMonth: '本月新增',
        inactive: '已停用',
        searchPlaceholder: '搜索消费者名称或邮箱',
        allTypes: '所有类型',
        allStatus: '所有状态',
        status: {
          active: '活跃',
          inactive: '已停用'
        },
        types: {
          enterprise: '企业版',
          premium: '高级版',
          standard: '标准版'
        },
        columns: {
          name: '名称',
          type: '类型',
          company: '公司',
          usage: '使用量',
          status: '状态',
          createdAt: '创建时间',
          actions: '操作'
        },
        noConsumers: '暂无消费者',
        addFirstConsumer: '添加第一位消费者开始使用',
        deleteConsumer: '删除消费者',
        deleteConfirm: '确定要删除消费者 {name} 吗？此操作不可恢复。',
        editNotImplemented: '编辑功能尚未实现',
        detailsNotImplemented: '详情功能尚未实现',
        consumerDeleted: '消费者已删除',
        fetchFailed: '获取消费者列表失败',
        createSuccess: '消费者创建成功',
        createNotImplemented: '创建功能尚未实现',
        createFailed: '创建消费者失败',
        editConsumer: '编辑消费者',
        editSuccess: '消费者更新成功',
        editFailed: '更新消费者失败',
        deleteFailed: '删除消费者失败',
        detailsTitle: '消费者详情',
        detailsFailed: '获取消费者详情失败',
        namePlaceholder: '请输入名称',
        nameRequired: '请输入名称',
        descriptionLabel: '描述',
        descriptionPlaceholder: '请输入描述',
        noDescription: '暂无描述',
        fields: {
          id: 'ID',
          teamId: '团队 ID',
          department: '部门',
          updatedAt: '更新时间'
        },
        offboardTitle: '注销消费者',
        offboardWarning: '此操作将永久禁用消费者',
        offboardWarningDetail: '确定要注销 {name} 吗？',
        affectedKeys: '受影响的 API 密钥',
        affectedKeysCount: '{count} 个 API 密钥将被禁用',
        noKeysFound: '该消费者暂无 API 密钥',
        consequences: '后果',
        consequenceKeysDisabled: '所有 API 密钥将被禁用',
        consequenceConsumerInactive: '消费者状态将被设为非活跃',
        consequenceIrreversible: '此操作无法撤销',
        confirmLabel: '输入 {name} 以确认',
        offboarding: '注销中...',
        confirmOffboard: '确认注销',
        confirmRequired: '需要确认',
        confirmMismatch: '确认文本不匹配'
      },
      analytics: {
        title: '团队统计',
        subtitle: '查看团队使用情况和性能指标',
        description: '查看团队统计数据',
        totalRequests: '总请求数',
        totalCost: '总成本',
        usageTrend: '使用趋势',
        platformDistribution: '平台分布',
        topConsumers: '消费排行',
        last7Days: '近7天',
        last30Days: '近30天',
        last90Days: '近90天',
        today: '今天',
        last3Days: '近3天',
        last15Days: '近15天',
        thisMonth: '本月',
        lastMonth: '上月',
        custom: '自定义',
        granularityDay: '按天',
        granularityHour: '按小时',
        viewConsumerDetails: '查看消费者详情',
        detailsNotImplemented: '详情功能尚未实现',
        fetchFailed: '获取统计数据失败',
        columns: {
          consumer: '消费者',
          requests: '请求数',
          cost: '成本',
          actions: '操作'
        }
      },
      settings: {
        title: '团队设置',
        subtitle: '管理团队基本信息、成员、账单及安全设置',
        tabs: {
          general: '通用设置',
          members: '成员管理',
          billing: '账单设置',
          danger: '危险区域'
        },
        general: {
          title: '基本信息',
          description: '设置团队名称、描述、头像、时区和语言偏好',
          teamName: '团队名称',
          teamNamePlaceholder: '请输入团队名称',
          descriptionLabel: '团队描述',
          descriptionPlaceholder: '请输入团队描述',
          teamAvatar: '团队头像',
          uploadAvatar: '上传头像',
          avatarHint: '支持 JPG、PNG 格式，建议尺寸 200x200',
          timezone: '时区',
          language: '语言偏好',
          lang: {
            zh: '简体中文',
            en: 'English'
          },
          saveSuccess: '通用设置已保存'
        },
        members: {
          inviteTitle: '邀请成员',
          inviteDescription: '通过邮箱邀请新成员加入团队',
          emailPlaceholder: '请输入成员邮箱',
          namePlaceholder: '请输入成员姓名（选填）',
          inviteButton: '邀请成员',
          inviteSuccess: '邀请已发送',
          listTitle: '成员列表',
          columns: {
            name: '姓名',
            email: '邮箱',
            role: '角色',
            status: '状态',
            joinedAt: '加入时间',
            actions: '操作'
          },
          roles: {
            owner: '所有者',
            member: '成员',
            manager: '经理',
            admin: '管理员',
            viewer: '访客'
          },
          roleUpdated: '{name} 的角色已更新',
          memberEnabled: '{name} 已启用',
          memberDisabled: '{name} 已禁用',
          memberRemoved: '{name} 已被移除',
          emailRequired: '请输入邮箱地址'
        },
        billing: {
          title: '账单设置',
          description: '配置支付方式、账单地址和发票设置',
          paymentMethod: '支付方式',
          methods: {
            creditCard: '信用卡',
            alipay: '支付宝',
            wechatPay: '微信支付',
            bankTransfer: '银行转账'
          },
          billingAddress: '账单地址',
          addressPlaceholder: '请输入账单地址',
          invoiceSettings: '发票设置',
          autoInvoice: '自动生成发票',
          invoiceEmail: '发送发票到邮箱',
          invoiceEmailAddress: '发票接收邮箱',
          invoiceEmailPlaceholder: '请输入发票接收邮箱',
          saveSuccess: '账单设置已保存'
        },
        danger: {
          title: '危险区域',
          description: '以下操作不可逆，请谨慎操作',
          transferOwnership: '转移所有权',
          transferOwnershipDesc: '将团队所有权转移给其他成员',
          transferButton: '转移所有权',
          transferDialogTitle: '确认转移所有权',
          transferDialogMessage: '确定要将团队所有权转移吗？此操作不可撤销。',
          transferSuccess: '所有权转移成功',
          deleteTeam: '删除团队',
          deleteTeamDesc: '删除团队将清除所有数据，此操作不可恢复',
          deleteButton: '删除团队',
          deleteDialogTitle: '确认删除团队',
          deleteDialogMessage: '确定要删除团队吗？此操作将清除所有数据且不可恢复！',
          deleteSuccess: '团队已删除'
        }
      }
    },
    tickets: {
      title: '工单管理',
      description: '查看、回复和处理用户提交的支持工单。',
      searchPlaceholder: '搜索标题或联系方式',
      empty: '暂无工单。',
      selectHint: '请选择左侧工单查看详情。',
      replyPlaceholder: '输入客服回复内容',
      statusUpdated: '工单状态已更新',
      statusUpdateFailed: '更新工单状态失败'
    },
    backup: {
      imageStorage: {
        title: '图片存储',
        description: '配置 S3 兼容对象存储，用于承载用户上传的图片（备份默认复用此配置）。',
        enabled: '启用图片存储',
        reuseBackupS3: '复用备份 S3 配置',
        bucketInherited: '继承自备份 S3 配置',
        publicBaseUrl: '公开访问基址',
        publicBaseUrlPlaceholder: '可选。用于在 UI 中渲染已上传图片的 CDN/源站 URL 前缀。',
        presignExpiryHours: '预签名有效期（小时）',
        saved: '图片存储配置已保存'
      },
      columns: {
        parts: '分片'
      },
      actions: {
        downloadFailed: '下载失败',
        downloadParts: '备份分片',
        downloadPartsHint: '超过 5GB 的备份会拆分为多个分片，需按序下载并拼接后才能用于恢复。',
        partLabel: '分片 {index}'
      }
    },
    users: {
      soraStorageQuota: 'Sora 存储配额',
      soraStorageQuotaHint: '单位 GB，0 表示使用分组或系统默认配额',
      bulkLimits: {
        title: '批量修改限额',
        selectUser: '已选择用户：{email}',
        selectedCount: '已选 {count} 个用户',
        apply: '应用',
        applying: '正在应用…',
        enableConcurrency: '切换并发',
        enableRPMLimit: '切换 RPM 限制',
        unlimited: '不限',
        nonNegativeInteger: '请输入非负整数（0 表示不限）',
        selectionLimit: '单次最多批量修改 {max} 个用户',
        concurrencyValue: '设置并发数为 {value}',
        rpmValue: '设置 RPM 为 {value}',
        rpmUnlimitedValue: '设置 RPM 为不限',
        confirm: '确认对 {count} 个用户应用修改？',
        success: '已成功更新 {count} 个用户',
        failed: '批量修改失败'
      }
    },
    groups: {
      duplicateSuccess: '已复制分组 "{name}"',
      usageYesterday: '昨日用量'
    },
    channels: {
      form: {
        codexImageGenerationBridgeHint: '开启后，OpenAI 分组的 Codex /responses 文本请求可能会被自动注入 image_generation 工具。仅在路由账号支持图片生成时开启。'
      }
    },
    governance: {
      title: 'AI 治理与合规',
      description: '面向 EU AI Act 与 GDPR 的治理能力：审计留痕、风险标签、评估报告与数据主体权利处理',
      refresh: '刷新',
      loadFailed: '加载失败',
      status: {
        primaryRole: '主体角色定位',
        riskTier: '风险等级',
        capabilities: '已启用能力'
      },
      capability: {
        risk_tagging: '风险标签',
        audit_logging: '审计留痕',
        gdpr_erasure: 'GDPR 删除',
        gdpr_data_export: '数据导出',
        consent_management: '同意管理',
        eu_ai_act_report: 'EU AI Act 报告'
      },
      tabs: {
        audit: '审计日志',
        risk: '风险分析',
        euAiAct: 'EU AI Act 评估',
        ropa: 'GDPR 处理记录',
        erasure: 'GDPR 删除请求',
        templates: '行业合规模板',
        rules: '内容审核规则',
        jurisdiction: '跨法域映射',
        dpa: 'DPA 协议',
        credentials: '合规凭证'
      },
      audit: {
        complianceType: '合规类型',
        complianceTypePlaceholder: '如 risk_assessment',
        subjectType: '主体类型',
        subjectTypePlaceholder: '如 user',
        subjectId: '主体 ID',
        details: '详情',
        operator: '操作者',
        createdAt: '时间',
        search: '查询',
        reset: '重置'
      },
      risk: {
        modelTags: '模型属性标签',
        riskTags: '合规风险标签'
      },
      euAiAct: {
        export: '导出报告',
        exportSuccess: '报告已导出',
        exportFailed: '导出失败',
        empty: '暂无评估报告，点击刷新生成'
      },
      ropa: {
        empty: '暂无数据处理记录，点击刷新生成'
      },
      erasure: {
        userId: '用户 ID',
        requestType: '请求类型',
        status: '状态',
        statusAll: '全部',
        statusPending: '待处理',
        statusApproved: '已批准',
        statusRejected: '已拒绝',
        statusCompleted: '已完成',
        requestedAt: '申请时间',
        actions: '操作',
        approve: '批准',
        reject: '拒绝',
        approveTitle: '批准删除请求',
        rejectTitle: '拒绝删除请求',
        confirmHint: '确认处理删除请求 #{id}？',
        reason: '拒绝原因',
        reasonPlaceholder: '请填写拒绝原因',
        reasonRequired: '拒绝时必须填写原因',
        processSuccess: '处理成功',
        processFailed: '处理失败'
      },
      templates: {
        empty: '暂无行业合规模板',
        create: '创建自定义模板',
        createTitle: '创建自定义合规模板',
        apply: '应用',
        active: '已启用',
        rules: '策略规则',
        applySuccess: '模板已应用，已记录合规审计',
        applyFailed: '应用模板失败',
        createSuccess: '模板已创建',
        createFailed: '创建模板失败',
        requiredFields: '模板编码与行业为必填项',
        invalidRules: '策略规则必须是合法的 JSON 数组',
        fieldCode: '模板编码',
        fieldIndustry: '行业',
        fieldDescription: '描述',
        fieldRules: '策略规则（JSON 数组）',
        fieldRiskTags: '风险标签',
        riskTagsPlaceholder: '多个标签用英文逗号分隔'
      },
      rules: {
        strategy: '策略组合',
        strategySuccess: '策略已更新',
        strategyFailed: '策略更新失败',
        create: '新建规则',
        createTitle: '新建审核规则',
        editTitle: '编辑审核规则',
        ruleId: '规则编码',
        ruleName: '规则名称',
        ruleType: '规则类型',
        rulePattern: '匹配模式',
        patternPlaceholder: 'KEYWORD 填关键词；REGEX 填正则；PATTERN 用 * 通配',
        threshold: '阈值/权重',
        priority: '优先级',
        riskCategory: '风险类目',
        action: '动作',
        enabled: '启用',
        disabled: '禁用',
        enabledLabel: '启用该规则',
        requiredFields: '规则编码、名称与匹配模式为必填项',
        saveSuccess: '规则已保存',
        saveFailed: '保存规则失败',
        deleteSuccess: '规则已删除',
        deleteFailed: '删除规则失败'
      },
      jurisdiction: {
        region: '公司注册法域',
        industry: '行业',
        industryPlaceholder: '如 healthcare / finance / ecommerce',
        serviceType: '服务类型',
        map: '生成映射',
        mapFailed: '生成跨法域映射失败',
        riskLevel: '风险等级',
        regulations: '适用法规',
        measures: '合规措施',
        actions: '建议行动',
        fieldHelp: '字段说明',
        fieldHelpRegion: '业务运营所在地区，用于识别适用的数据保护法规（如GDPR、中国个人信息保护法等）',
        fieldHelpIndustry: '所属行业，影响风险评估等级和合规要求（如医疗、金融、教育等行业有特殊要求）',
        fieldHelpServiceType: 'AI服务类型，帮助系统识别具体的合规义务和评估标准。选项说明：AI Chatbot（AI聊天机器人服务）、AI Analysis（AI数据分析服务）、AI Recommendation（AI推荐服务）'
      },
      dpa: {
        title: 'DPA 合规声明',
        controllerName: '数据控制者名称',
        controllerNamePlaceholder: '输入数据控制者公司名称',
        controllerContact: '数据控制者联系人',
        controllerContactPlaceholder: '输入联系人邮箱或电话',
        generate: '生成 DPA',
        requiredFields: '请填写所有必填字段',
        generateSuccess: 'DPA 合规声明生成成功',
        generateFailed: 'DPA 合规声明生成失败'
      },
      credentials: {
        status: '状态',
        create: '创建凭证',
        credentialId: '凭证 ID',
        type: '类型',
        issuer: '颁发者',
        validUntil: '有效期至',
        createdAt: '创建时间',
        revoke: '吊销',
        revokeSuccess: '凭证已吊销',
        revokeFailed: '吊销凭证失败',
        activate: '激活',
        activateSuccess: '凭证已激活',
        activateFailed: '激活凭证失败',
        deleteSuccess: '凭证已删除',
        deleteFailed: '删除凭证失败',
        loadFailed: '加载凭证列表失败',
        requiredFields: '请填写所有必填字段',
        createSuccess: '凭证创建成功',
        createFailed: '创建凭证失败'
      }
    },
    dataRights: {
      title: '数据主体权利',
      description: '管理您的数据主体权利：导出个人数据、请求删除数据、管理同意记录。',
      export: {
        title: '数据导出',
        description: '根据 GDPR 第 20 条，您可以导出您的个人数据。',
        button: '导出数据',
        processing: '正在处理...',
        success: '导出请求已提交，导出 ID：{id}',
        successMessage: '数据导出请求已提交',
        error: '导出数据失败'
      },
      erasure: {
        title: '数据删除',
        description: '根据 GDPR 第 17 条，您可以请求删除您的个人数据。',
        reasonLabel: '删除原因',
        reasonPlaceholder: '请说明您请求删除数据的原因...',
        confirmLabel: '确认文本',
        confirmPlaceholder: '请输入 "DELETE MY DATA" 以确认',
        confirmHint: '请输入 DELETE MY DATA 以确认删除操作',
        submit: '提交删除请求',
        submitting: '正在提交...',
        success: '删除请求已提交，请求 ID：{id}',
        successMessage: '数据删除请求已提交',
        error: '提交删除请求失败',
        history: '删除请求历史',
        historyDescription: '查看您提交的数据删除请求状态。',
        noRequests: '暂无删除请求',
        request: '删除请求',
        requestType: '请求类型',
        reason: '原因',
        status: {
          pending: '待处理',
          approved: '已批准',
          rejected: '已拒绝',
          completed: '已完成'
        }
      },
      consent: {
        title: '同意记录',
        description: '查看和管理您的数据处理同意记录。',
        empty: '暂无同意记录',
        version: '版本',
        status: '状态',
        granted: '已同意',
        revoked: '已撤销',
        grantedAt: '同意时间',
        loadError: '加载同意记录失败',
        updateSuccess: '同意记录已更新',
        updateError: '更新同意记录失败',
        types: {
          terms_of_service: {
            label: '服务条款',
            description: '我已阅读并同意服务条款（必选项）'
          },
          gdpr_data_processing: {
            label: 'GDPR 数据处理协议',
            description: '我同意根据 GDPR 数据处理协议处理我的个人数据（必选项）'
          },
          detailed_logging: {
            label: '详细日志记录',
            description: '允许系统记录详细的请求和响应日志用于审计和故障排查'
          },
          cross_border_transfer: {
            label: '跨境数据传输',
            description: '允许数据在不同国家/地区之间传输以提供服务'
          },
          marketing: {
            label: '营销信息',
            description: '接收产品更新、促销活动和其他营销信息'
          },
          model_training: {
            label: '模型训练数据',
            description: '允许您的数据用于AI模型改进和优化'
          }
        }
      }
    },
    compliance: {
      title: 'Account 合规配置',
      description: '配置您的 Account 级 AI 治理与合规策略，包括行业模板、ZDR 模式、合规框架和内容审核规则。',
      template: {
        title: '行业模板',
        description: '选择适合您行业的合规模板，快速应用预定义的合规策略。',
        apply: '应用模板',
        current: '当前模板',
        industries: {
          ecommerce: {
            label: '电子商务',
            description: '电商行业合规模板：推荐引擎用户画像告知、数据保留 90 天。'
          },
          finance: {
            label: '金融服务',
            description: '金融行业合规模板：信用评分人工监督、审计留痕、反欺诈、数据保留 365 天。'
          },
          healthcare: {
            label: '医疗健康',
            description: '医疗行业合规模板：医疗建议人工监督、HIPAA 合规、患者数据保护、数据保留 730 天。'
          },
          education: {
            label: '教育培训',
            description: '教育行业合规模板：学习评估人工监督、未成年人数据保护、数据保留 180 天。'
          }
        }
      },
      zdr: {
        title: 'ZDR 设置',
        description: '配置零数据残留（Zero Data Retention）模式，控制数据保留策略。',
        mode: 'ZDR 模式',
        aggregate_only: '仅聚合（Aggregate Only）',
        audit: '审计（Audit）',
        retention_days: '明细日志保留天数'
      },
      frameworks: {
        title: '合规框架',
        description: '选择适用的合规框架，确保您的 Account 符合相关法规要求。',
        gdpr: 'GDPR',
        eu_ai_act: 'EU AI Act',
        hipaa: 'HIPAA'
      },
      moderation: {
        title: '内容审核策略',
        description: '管理内容审核规则，启用或停用特定的审核策略。',
        enabled: '已启用'
      },
      customRules: {
        title: '自定义审核规则',
        description: '创建属于您自己的内容审核规则，用于补充系统默认规则。',
        create: '新建规则',
        createTitle: '新建自定义规则',
        editTitle: '编辑自定义规则',
        edit: '编辑',
        delete: '删除',
        update: '保存',
        enabled: '启用',
        disabled: '停用',
        enableRule: '启用规则',
        empty: '暂无自定义规则',
        ruleName: '规则名称',
        ruleNamePlaceholder: '如：内部禁用词',
        ruleType: '规则类型',
        rulePattern: '匹配模式',
        patternPlaceholder: '如：禁用词|敏感词（REGEX 用 | 分隔多个关键词）',
        action: '动作',
        riskCategory: '风险类别',
        riskCategoryPlaceholder: '如：自定义',
        createSuccess: '自定义规则创建成功',
        updateSuccess: '自定义规则更新成功',
        deleteSuccess: '自定义规则删除成功',
        deleteConfirm: '确定删除这条自定义规则吗？'
      },
      status: {
        title: '配置状态'
      },
      jurisdiction: {
        title: '跨法域映射',
        description: '根据您的业务区域、行业和服务类型，自动映射适用的合规法规和要求。',
        region: '公司区域',
        industry: '行业',
        industryPlaceholder: '如：医疗健康、金融服务',
        serviceType: '服务类型',
        map: '开始映射',
        save: '保存配置',
        applyRules: '自动应用合规规则',
        saved: '配置已保存',
        appliedRules: '已应用规则',
        saveSuccess: '配置保存成功',
        riskLevel: '风险等级',
        regulations: '适用法规',
        measures: '必需措施',
        actions: '建议行动',
        fieldHelp: '字段说明',
        fieldHelpRegion: '业务运营所在地区，用于识别适用的数据保护法规（如GDPR、中国个人信息保护法等）',
        fieldHelpIndustry: '所属行业，影响风险评估等级和合规要求（如医疗、金融、教育等行业有特殊要求）',
        fieldHelpServiceType: 'AI服务类型，帮助系统识别具体的合规义务和评估标准。选项说明：AI Chatbot（AI聊天机器人服务）、AI Analysis（AI数据分析服务）、AI Recommendation（AI推荐服务）'
      },
      dpa: {
        title: 'DPA 合规声明',
        description: '生成数据处理协议合规声明（DPA Compliance Statement），用于展示 GDPR Art.28 合规要素。此文件为合规声明，非可签署的法律合同。',
        controllerName: '数据控制方名称',
        controllerNamePlaceholder: '输入您的公司名称',
        controllerContact: '数据控制方联系人',
        controllerContactPlaceholder: '输入联系人邮箱或电话',
        generate: '生成 DPA',
        success: 'DPA 合规声明生成成功，文件已下载。'
      },
      credentials: {
        title: '合规凭证',
        description: '查看您的合规凭证列表，包括认证证书、审计报告等。',
        empty: '暂无合规凭证',
        validFrom: '有效期开始',
        validUntil: '有效期结束',
        scope: '适用范围',
        issuerType: '颁发类型',
        credentialTypes: {
          GDPR_COMPLIANCE: 'GDPR 合规凭证',
          EU_AI_ACT_ASSESSMENT: 'EU AI Act 评估报告',
          ZERO_DATA_RETENTION: '零数据保留声明',
          DPA_COMPLIANCE: '数据处理协议（DPA）',
          SECURITY_CERTIFICATION: '安全认证'
        },
        issuerTypes: {
          SELF_ASSERTION: '自我声明',
          THIRD_PARTY: '第三方认证'
        },
        metadata: {
          title: '详细信息',
          compliance_basis: '合规依据',
          data_processing_record: '处理活动记录',
          dpo_contact: 'DPO 联系方式',
          data_retention: '数据保留策略',
          assessment_date: '评估日期',
          risk_category: '风险类别',
          human_in_the_loop: '人工干预',
          model_training: '模型训练',
          prompt_storage: '提示词存储',
          policy_version: '策略版本',
          request_content: '请求内容',
          technical_logs: '技术日志',
          security_logs: '安全日志',
          backup_policy: '备份策略',
          dpa_version: 'DPA 版本',
          scc_compliant: 'SCC 合规',
          subprocessor_approval_required: '需批准子处理者',
          data_subject_rights_supported: '支持的数据主体权利',
          encryption_at_rest: '静态加密',
          transport_security: '传输安全',
          access_control: '访问控制',
          audit_logging: '审计日志',
          security_audits: '安全审计'
        }
      },
      audit: {
        title: '审计日志',
        description: '查看您的合规操作审计日志，记录所有合规相关活动。',
        empty: '暂无审计日志',
        operator: '操作人',
        page: '第',
        prev: '上一页',
        next: '下一页'
      },
      risk: {
        title: '风险分析',
        description: '查看系统支持的风险标签目录，了解模型风险和数据风险分类。',
        modelTags: '模型标签',
        riskTags: '风险标签'
      },
      euAiAct: {
        title: 'EU AI Act 评估',
        description: '查看您的 AI 系统符合 EU AI Act 的评估报告。',
        export: '导出评估报告',
        exportSuccess: '评估报告导出成功。'
      },
      ropa: {
        title: 'GDPR 处理记录',
        description: '查看 GDPR Art 30 数据处理活动记录（ROPA），满足合规审计要求。'
      }
    },
    subscriptions: {
      daysRemaining: '天剩余',
      revokeConfirm: '确定要撤销 \'{user}\' 的订阅吗？此操作无法撤销。',
      guide: {
        actions: {
          revokeDesc: '立即终止该用户的订阅，不可恢复'
        }
      }
    },
    accounts: {
      deleteConfirmMessage: '确定要删除账号 \'{name}\' 吗？',
      refreshCookie: '刷新 Cookie',
      testAccount: '测试账号',
      types: {
        api_key: 'API Key',
        cookie: 'Cookie'
      },
      openaiQuotaReset: {
        resetSuccess: '已重置 {windows} 个窗口'
      },
      form: {
        nameLabel: '账号名称',
        namePlaceholder: '请输入账号名称',
        platformLabel: '平台',
        selectPlatform: '选择平台',
        typeLabel: '类型',
        selectType: '选择类型',
        credentialsLabel: '凭证',
        credentialsPlaceholder: '请输入 Cookie 或 API Key',
        priorityLabel: '优先级',
        priorityHint: '数值越小优先级越高',
        weightLabel: '权重',
        weightHint: '用于负载均衡的权重值',
        statusLabel: '状态'
      },
      filters: {
        platform: '平台',
        allPlatforms: '全部平台',
        type: '类型',
        allTypes: '全部类型',
        status: '状态',
        allStatuses: '全部状态'
      },
      saving: '保存中...',
      refreshing: '刷新中...',
      noAccounts: '暂无账号',
      noAccountsDescription: '添加 AI 平台账号以开始使用 API 网关。',
      accountCreatedSuccess: '账号添加成功',
      accountUpdatedSuccess: '账号更新成功',
      accountDeletedSuccess: '账号删除成功',
      bulkEdit: {
        baseUrlNotice: '仅适用于 API Key 账号，留空则不修改'
      },
      cookieRefreshedSuccess: 'Cookie 刷新成功',
      testSuccess: '账号测试通过',
      failedToSave: '保存账号失败',
      openai: {
        wsModeDesc: '仅对当前 OpenAI 账号类型生效。',
        codexImageGenerationBridge: 'Codex 图片生成桥接',
        codexImageGenerationBridgeDesc: '账号级策略优先于渠道和全局配置。仅控制 Codex 走 /responses 文本端点时是否注入 image_generation 工具；不影响独立图片生成接口。',
        codexImageGenerationBridgeInherit: '跟随渠道',
        codexImageGenerationBridgeInheritDesc: '不写入账号覆盖，继续使用渠道或全局策略。',
        codexImageGenerationBridgeEnabled: '强制开启',
        codexImageGenerationBridgeEnabledDesc: '允许 Codex /responses 请求获得图片工具注入。',
        codexImageGenerationBridgeDisabled: '强制关闭',
        codexImageGenerationBridgeDisabledDesc: '阻断 Codex /responses 的图片工具注入。',
        codexImageGenerationBridgeBadgeInherit: '渠道策略',
        codexImageGenerationBridgeBadgeEnabled: '账号开启',
        codexImageGenerationBridgeBadgeDisabled: '账号关闭'
      },
      oauth: {
        openai: {
          codexSessionAuth: 'Codex JSON / AT 批量输入',
          codexSessionDesc: '粘贴 Codex JSON 或 accessToken，按第一步配置创建账号。',
          codexSessionInputLabel: 'Codex JSON 或 accessToken',
          codexSessionPlaceholder: '支持多行，每行一个 token 或 JSON',
          codexSessionHint: 'sessionToken 不会作为 refresh_token 保存；未包含 refresh_token 时会按 accessToken 过期时间设置账号过期，无法解析且第一步未设置过期时间时会拒绝导入。',
          codexSessionEmpty: '请输入 Codex JSON 或 accessToken'
        }
      },
      imageTestHint: '选择图片模型后，这里会直接发起生图测试，并在下方展示返回图片。'
    },
    proxies: {
      batchInputPlaceholder: '每行输入一个代理，支持以下格式：\nsocks5://user:pass 192.168.1.1:1080\nhttp://192.168.1.1:8080\nhttps://user:pass&#64;proxy.example.com:443',
      batchInputHint: '支持 http、https、socks5 协议，格式：协议://[用户名:密码&#64;]主机:端口',
      fallbackMode: '失败回退'
    },
    usage: {
      inputCost: '输入成本',
      outputCost: '输出成本',
      cacheCreationCost: '缓存创建成本',
      cacheReadCost: '缓存读取成本',
      department: '部门',
      consumer: '消费者',
      departmentPlaceholder: '按部门名称筛选',
      consumerPlaceholder: '按消费者名称筛选'
    },
    ops: {
      errorDetail: {
        attemptedKeyPrefix: '尝试的 Key 前缀',
        deletedKeyOwner: '已删除 Key 所有者'
      },
      settings: {
        ignoreInvalidApiKeyErrors: '忽略无效 API Key 错误',
        ignoreInvalidApiKeyErrorsHint: '启用后，无效或缺失 API Key 的错误（INVALID_API_KEY、API_KEY_REQUIRED）将不会写入错误日志。'
      }
    },
    settings: {
      features: {
        channelMonitor: {
          description: '定期对配置的渠道发起健康检查，向用户展示可用性与延迟。关闭后调度器停止扫描，用户端列表为空。',
          enabledHint: '关闭后后台不再执行定时检测，已有数据保留。',
          defaultIntervalHint: '新建渠道监控时表单的默认值，可被单个渠道覆盖。范围 15 – 3600 秒。'
        }
      },
      registration: {
        emailSuffixWhitelistHint: '仅允许使用指定域名的邮箱注册账号（例如 &#64;qq.com, &#64;gmail.com, *.edu.cn）',
        emailSuffixWhitelistPlaceholder: '&#64;example.com, *.edu.cn'
      },
      apiKeyAcl: {
        description: '控制 API Key 白名单和黑名单使用哪个客户端 IP 判断',
        trustForwardedIpHint: '默认关闭。仅在源站只允许 Cloudflare 或 Nginx 反代访问时开启；开启后 API Key IP 白/黑名单会使用 CF-Connecting-IP、X-Real-IP 或 X-Forwarded-For，与使用记录中的请求 IP 保持一致。'
      },
      gatewayForwarding: {
        claudeOAuthSystemPromptBlocksPlaceholder: "留空时使用内置 3 个 blocks。支持数组或 {'{'}\"blocks\": [...]{'}'}。",
        claudeOAuthSystemPromptBlocksHint: '每个 block 会保存为带 enabled、type、text、可选 cache_control 的 JSON。{billing_header} 会按请求动态生成；Claude Code 身份提示词和扩展提示词可直接编辑，也可用预设恢复默认值。'
      },
      soraClient: {
        title: 'Sora 客户端',
        description: '控制是否在侧边栏展示 Sora 客户端入口',
        enabled: '启用 Sora 客户端',
        enabledHint: '开启后，侧边栏将显示 Sora 入口，用户可访问 Sora 功能'
      },
      payment: {
        alipayGuideSummary: '桌面优先扫码单，失败再走收银台；移动优先手机网站支付。',
        alipayGuideWapCall: '移动端优先调用 alipay.trade.wap.pay，跳转支付宝收银台。'
      },
      smtp: {
        usernamePlaceholder: 'your-email&#64;gmail.com',
        fromEmailPlaceholder: 'noreply&#64;example.com'
      },
      testEmail: {
        recipientEmailPlaceholder: 'test&#64;example.com'
      },
      soraS3: {
        title: 'Sora 存储配置',
        description: '以多配置列表管理 Sora 媒体存储，支持 S3 和 Google Drive',
        newProfile: '新建配置',
        reloadProfiles: '刷新列表',
        empty: '暂无存储配置，请先创建',
        createTitle: '新建存储配置',
        editTitle: '编辑存储配置',
        selectProvider: '选择存储类型',
        providerS3Desc: 'S3 兼容对象存储',
        providerGDriveDesc: 'Google Drive 云盘',
        profileID: '配置 ID',
        profileName: '配置名称',
        setActive: '创建后设为生效',
        saveProfile: '保存配置',
        activateProfile: '设为生效',
        profileCreated: '存储配置创建成功',
        profileSaved: '存储配置保存成功',
        profileDeleted: '存储配置删除成功',
        profileActivated: '生效配置已切换',
        profileIDRequired: '请填写配置 ID',
        profileNameRequired: '请填写配置名称',
        profileSelectRequired: '请先选择配置',
        endpointRequired: '启用时必须填写 S3 端点',
        bucketRequired: '启用时必须填写存储桶',
        accessKeyRequired: '启用时必须填写 Access Key ID',
        deleteConfirm: '确定删除存储配置 {profileID} 吗？',
        columns: {
          profile: '配置',
          profileId: 'Profile ID',
          name: '名称',
          provider: '存储类型',
          active: '生效状态',
          endpoint: '端点',
          bucket: '存储桶',
          storagePath: '存储路径',
          capacityUsage: '容量 / 已用',
          capacityUnlimited: '无限制',
          videoCount: '视频数',
          videoCompleted: '完成',
          videoInProgress: '进行中',
          quota: '默认配额',
          updatedAt: '更新时间',
          actions: '操作',
          rootFolder: '根目录',
          testInTable: '测试',
          testingInTable: '测试中...',
          testTimeout: '测试超时（15秒）'
        },
        enabled: '启用存储',
        enabledHint: '启用后，Sora 生成的媒体文件将自动上传到存储',
        endpoint: 'S3 端点',
        region: '区域',
        bucket: '存储桶',
        prefix: '对象前缀',
        accessKeyId: 'Access Key ID',
        secretAccessKey: 'Secret Access Key',
        secretConfigured: '(已配置，留空保持不变)',
        cdnUrl: 'CDN URL',
        cdnUrlHint: '可选，配置后使用 CDN URL 访问文件',
        forcePathStyle: '强制路径风格（Path Style）',
        defaultQuota: '默认存储配额',
        defaultQuotaHint: '未在用户或分组级别指定配额时的默认值，0 表示无限制',
        testConnection: '测试连接',
        testing: '测试中...',
        testSuccess: '连接测试成功',
        testFailed: '连接测试失败',
        saved: '存储设置保存成功',
        saveFailed: '保存存储设置失败',
        gdrive: {
          authType: '认证方式',
          serviceAccount: '服务账号',
          clientId: 'Client ID',
          clientSecret: 'Client Secret',
          clientSecretConfigured: '(已配置，留空保持不变)',
          refreshToken: 'Refresh Token',
          refreshTokenConfigured: '(已配置，留空保持不变)',
          serviceAccountJson: '服务账号 JSON',
          serviceAccountConfigured: '(已配置，留空保持不变)',
          folderId: 'Folder ID（可选）',
          authorize: '授权 Google Drive',
          authorizeHint: '通过 OAuth2 获取 Refresh Token',
          oauthFieldsRequired: '请先填写 Client ID 和 Client Secret',
          oauthSuccess: 'Google Drive 授权成功',
          oauthFailed: 'Google Drive 授权失败',
          closeWindow: '此窗口将自动关闭',
          processing: '正在处理授权...',
          testStorage: '测试存储',
          testSuccess: 'Google Drive 存储测试成功（上传、访问、删除均正常）',
          testFailed: 'Google Drive 存储测试失败'
        }
      }
      // openaiFastPolicy 已删除：上游 locales/zh/admin/settings.ts 的新版文案
      // （目标模型/其他模型处理方式）才是当前 UI 所需，旧 fork 副本已过时。
    }
  },
  team: {
    consumer: {
      createTitle: '创建消费者',
      editTitle: '编辑消费者',
      name: '名称',
      namePlaceholder: '请输入消费者名称',
      email: '邮箱',
      emailPlaceholder: '请输入消费者邮箱',
      phone: '电话',
      phonePlaceholder: '请输入消费者电话',
      title: '职位',
      titlePlaceholder: '请输入消费者职位',
      type: '类型',
      department: '部门',
      selectDepartment: '选择部门',
      noDepartments: '暂无部门',
      status: '状态',
      statusActive: '活跃',
      statusInactive: '已停用',
      types: {
        person: '个人',
        application: '应用',
        serviceAccount: '服务账号'
      },
      errors: {
        nameRequired: '名称为必填项',
        typeRequired: '类型为必填项',
        departmentRequired: '部门为必填项'
      }
    }
  },
  payment: {
    admin: {
      validityDays: '有效期（天）',
      validityDaysRequired: '有效期天数必须大于 0'
    }
  },
  legalDocument: {
    login: '登录',
    loginTerms: '登录条款',
    loadError: '文档加载失败',
    loadErrorDesc: '请稍后刷新页面重试。',
    notFound: '文档不存在',
    notFoundDesc: '当前条款文档不存在或已被管理员移除。',
    updatedAt: '更新日期：',
    noContent: '暂无正文内容',
    terms: '服务条款',
    "usage-policy": '使用政策',
    "supported-regions": '支持的国家和地区',
    "service-specific-terms": '服务特定条款'
  },
  governance: {
    title: 'AI 治理与合规',
    description: '管理您的 AI 治理与合规设置。'
  },
  dataRights: {
    title: '数据主体权利',
    description: '管理您的数据主体权利：导出个人数据、请求删除数据、管理同意记录。',
    export: {
      title: '数据导出',
      description: '根据 GDPR 第 20 条，您可以导出您的个人数据。',
      button: '导出数据',
      processing: '处理中...',
      success: '导出请求已提交，导出 ID：{id}',
      successMessage: '数据导出请求已提交',
      error: '数据导出失败'
    },
    erasure: {
      title: '数据删除',
      description: '根据 GDPR 第 17 条，您可以请求删除您的个人数据。',
      reasonLabel: '原因',
      reasonPlaceholder: '请解释您希望删除数据的原因...',
      confirmLabel: '确认文本',
      confirmPlaceholder: '请输入 "DELETE MY DATA" 以确认',
      confirmHint: '输入 DELETE MY DATA 以确认删除',
      submit: '提交删除请求',
      submitting: '提交中...',
      success: '删除请求已提交，请求 ID：{id}',
      successMessage: '数据删除请求已提交',
      error: '提交删除请求失败',
      history: '删除请求历史',
      historyDescription: '查看您提交的数据删除请求状态。',
      noRequests: '暂无删除请求',
      request: '删除请求',
      requestType: '请求类型',
      reason: '原因',
      status: {
        pending: '待处理',
        approved: '已批准',
        rejected: '已拒绝',
        completed: '已完成'
      }
    },
    consent: {
      title: '同意记录',
      description: '查看和管理您的数据处理同意记录。',
      empty: '暂无同意记录',
      version: '版本',
      status: '状态',
      granted: '已授予',
      revoked: '已撤销',
      grantedAt: '授予时间',
      createdAt: '创建时间',
      loadError: '加载同意记录失败',
      updateSuccess: '同意记录已更新',
      updateError: '更新同意记录失败',
      types: {
        terms_of_service: {
          label: '服务条款',
          description: '我已阅读并同意服务条款（必选项）'
        },
        gdpr_data_processing: {
          label: 'GDPR 数据处理协议',
          description: '我同意根据 GDPR 数据处理协议处理我的个人数据（必选项）'
        },
        detailed_logging: {
          label: '详细日志记录',
          description: '允许系统记录详细的请求和响应日志用于审计和故障排查'
        },
        cross_border_transfer: {
          label: '跨境数据传输',
          description: '允许数据在不同国家/地区之间传输以提供服务'
        },
        marketing: {
          label: '营销信息',
          description: '接收产品更新、促销活动和其他营销信息'
        },
        model_training: {
          label: '模型训练数据',
          description: '允许您的数据用于AI模型改进和优化'
        }
      }
    }
  },
  compliance: {
    title: '账户合规配置',
    description: '配置账户级别的 AI 治理与合规策略，包括行业模板、ZDR 模式、合规框架和内容审核规则。',
    template: {
      title: '行业模板',
      description: '选择适合您行业的合规模板，快速应用预定义的合规策略。',
      apply: '应用模板',
      current: '当前模板',
      notApplied: '未应用',
      industries: {
        ecommerce: {
          label: '电子商务',
          description: '电商行业合规模板：推荐引擎用户画像告知、数据保留 90 天。'
        },
        finance: {
          label: '金融服务',
          description: '金融行业合规模板：信用评分人工监督、审计留痕、反欺诈、数据保留 365 天。'
        },
        healthcare: {
          label: '医疗健康',
          description: '医疗行业合规模板：医疗建议人工监督、HIPAA 合规、患者数据保护、数据保留 730 天。'
        },
        education: {
          label: '教育培训',
          description: '教育行业合规模板：学习评估人工监督、未成年人数据保护、数据保留 180 天。'
        }
      }
    },
    zdr: {
      title: 'ZDR 设置',
      description: '配置零数据保留（ZDR）模式，控制数据保留策略。',
      mode: 'ZDR 模式',
      aggregate_only: '仅聚合',
      audit: '审计模式',
      retention_days: '明细日志保留天数'
    },
    frameworks: {
      title: '合规框架',
      description: '选择适用的合规框架，确保您的账户符合相关监管要求。',
      gdpr: 'GDPR',
      eu_ai_act: 'EU AI Act',
      hipaa: 'HIPAA',
      active: '个生效'
    },
    moderation: {
      title: '内容审核策略',
      description: '管理内容审核规则，启用或禁用特定的审核策略。',
      enabled: '已启用',
      enabledRules: '条规则生效'
    },
    customRules: {
      title: '自定义审核规则',
      description: '创建属于您自己的内容审核规则，用于补充系统默认规则。',
      create: '新建规则',
      createTitle: '新建自定义规则',
      editTitle: '编辑自定义规则',
      edit: '编辑',
      delete: '删除',
      update: '保存',
      enabled: '启用',
      disabled: '停用',
      enableRule: '启用规则',
      empty: '暂无自定义规则',
      ruleName: '规则名称',
      ruleNamePlaceholder: '如：内部禁用词',
      ruleType: '规则类型',
      rulePattern: '匹配模式',
      patternPlaceholder: '如：禁用词|敏感词（REGEX 用 | 分隔多个关键词）',
      action: '动作',
      riskCategory: '风险类别',
      riskCategoryPlaceholder: '如：自定义',
      createSuccess: '自定义规则创建成功',
      updateSuccess: '自定义规则更新成功',
      deleteSuccess: '自定义规则删除成功',
      deleteConfirm: '确定删除这条自定义规则吗？'
    },
    status: {
      title: '配置状态',
      description: '查看您账户的合规配置生效状态概览。'
    },
    jurisdiction: {
      title: '跨法域映射',
      description: '根据您的业务区域、行业和服务类型，自动映射适用的合规法规和要求。',
      region: '公司区域',
      industry: '行业',
      industryPlaceholder: '如：医疗健康、金融服务',
      serviceType: '服务类型',
      map: '开始映射',
      riskLevel: '风险等级',
      regulations: '适用法规',
      measures: '必需措施',
      actions: '建议行动',
      fieldHelp: '字段说明',
      fieldHelpRegion: '业务运营所在地区，用于识别适用的数据保护法规（如GDPR、中国个人信息保护法等）',
      fieldHelpIndustry: '所属行业，影响风险评估等级和合规要求（如医疗、金融、教育等行业有特殊要求）',
      fieldHelpServiceType: 'AI服务类型，帮助系统识别具体的合规义务和评估标准。选项说明：AI Chatbot（AI聊天机器人服务）、AI Analysis（AI数据分析服务）、AI Recommendation（AI推荐服务）',
      applyRules: '自动应用到合规规则',
      save: '保存映射结果',
      saved: '映射已保存',
      appliedRules: '已应用规则',
      saveSuccess: '映射保存成功',
      applied: '已应用',
      notApplied: '未应用'
    },
    dpa: {
      title: 'DPA 协议',
      description: '生成数据处理协议（DPA），用于满足 GDPR 等法规要求的数据处理合同。',
      controllerName: '数据控制方名称',
      controllerNamePlaceholder: '输入您的公司名称',
      controllerContact: '数据控制方联系人',
      controllerContactPlaceholder: '输入联系人邮箱或电话',
      generate: '生成 DPA',
      success: 'DPA 生成成功，文件已下载。'
    },
    credentials: {
      title: '合规凭证',
      description: '查看您的合规凭证列表，包括认证证书、审计报告等。',
      empty: '暂无合规凭证',
      validFrom: '有效期开始',
      validUntil: '有效期结束',
      scope: '适用范围',
      issuerType: '颁发类型',
      credentialTypes: {
        GDPR_COMPLIANCE: 'GDPR 合规凭证',
        EU_AI_ACT_ASSESSMENT: 'EU AI Act 评估报告',
        ZERO_DATA_RETENTION: '零数据保留声明',
        DPA_COMPLIANCE: '数据处理协议（DPA）',
        SECURITY_CERTIFICATION: '安全认证'
      },
      issuerTypes: {
        SELF_ASSERTION: '自我声明',
        THIRD_PARTY: '第三方认证'
      },
      metadata: {
        title: '详细信息',
        compliance_basis: '合规依据',
        data_processing_record: '处理活动记录',
        dpo_contact: 'DPO 联系方式',
        data_retention: '数据保留策略',
        assessment_date: '评估日期',
        risk_category: '风险类别',
        human_in_the_loop: '人工干预',
        model_training: '模型训练',
        prompt_storage: '提示词存储',
        policy_version: '策略版本',
        request_content: '请求内容',
        technical_logs: '技术日志',
        security_logs: '安全日志',
        backup_policy: '备份策略',
        dpa_version: 'DPA 版本',
        scc_compliant: 'SCC 合规',
        subprocessor_approval_required: '需批准子处理者',
        data_subject_rights_supported: '支持的数据主体权利',
        encryption_at_rest: '静态加密',
        transport_security: '传输安全',
        access_control: '访问控制',
        audit_logging: '审计日志',
        security_audits: '安全审计'
      }
    },
    audit: {
      title: '审计日志',
      description: '查看您的合规操作审计日志，记录所有合规相关活动。',
      empty: '暂无审计日志',
      operator: '操作人',
      page: '第',
      prev: '上一页',
      next: '下一页'
    },
    risk: {
      title: '风险分析',
      description: '查看系统支持的风险标签目录，了解模型风险和数据风险分类。',
      modelTags: '模型标签',
      riskTags: '风险标签',
      tags: {
        MODEL_FRONTIER: {
          label: '前沿模型',
          description: '使用前沿模型'
        },
        MODEL_OPEN_SOURCE: {
          label: '开源模型',
          description: '使用开源模型'
        },
        MODEL_EXTERNAL_PROVIDER: {
          label: '外部提供者',
          description: '使用外部提供者模型'
        },
        MODEL_DATA_RETENTION_UNKNOWN: {
          label: '数据保留策略未知',
          description: '模型提供者数据保留策略未知'
        },
        PII_DETECTED: {
          label: '检测到个人身份信息',
          description: '检测到个人身份信息'
        },
        HIGH_RISK_USE_CASE: {
          label: '高风险应用场景',
          description: '高风险应用场景'
        },
        CROSS_BORDER_TRANSFER: {
          label: '跨境数据传输',
          description: '跨境数据传输'
        },
        SANCTIONED_REGION: {
          label: '制裁区域访问',
          description: '制裁区域访问'
        },
        CONTENT_POLICY_VIOLATION: {
          label: '内容政策违规',
          description: '内容政策违规'
        },
        OUTPUT_CONTROL_LIMITED: {
          label: '输出控制受限',
          description: '输出控制受限'
        },
        NO_TRAINING_GUARANTEE: {
          label: '无训练数据保障',
          description: '无训练数据保障'
        },
        RATE_LIMIT_EXCEEDED: {
          label: '超限调用',
          description: '超限调用'
        },
        ANOMALOUS_BEHAVIOR: {
          label: '异常行为',
          description: '异常行为'
        }
      }
    },
    euAiAct: {
      title: 'EU AI Act 评估',
      description: '查看您的 AI 系统符合 EU AI Act 的评估报告。',
      export: '导出评估报告',
      exportSuccess: '评估报告导出成功。'
    },
    ropa: {
      title: 'GDPR 处理记录',
      description: '查看 GDPR Art 30 数据处理活动记录（ROPA），满足合规审计要求。'
    },
    login: '登录',
    hero: {
      badge: '企业级AI合规解决方案',
      title: '合规先行，AI无忧',
      subtitle: 'ThreeRouter提供完整的AI治理与合规框架，助您从容应对EU AI Act、GDPR等全球法规要求。',
      ctaPrimary: '立即体验',
      ctaSecondary: '了解更多'
    },
    features: {
      euai: {
        title: 'EU AI Act 合规评估',
        description: '专业的AI系统角色定位与法律映射，基于Annex III的高风险场景评估。',
        highlight1: 'AI系统角色定位与法律映射',
        highlight2: '基于Annex III的高风险场景评估',
        highlight3: 'GPAI下游集成商合规框架',
        highlight4: 'Article 50透明度义务完整披露'
      },
      gdpr: {
        title: 'GDPR Art.30 ROPA',
        description: '完整的处理活动记录（Processor Activities），合规的合法依据声明。',
        highlight1: '完整的处理活动记录',
        highlight2: '合规的合法依据声明',
        highlight3: 'EU SCC跨境传输保障机制',
        highlight4: '数据主体权利支持'
      },
      zdr: {
        title: '零数据保留架构',
        description: '默认不保留请求内容（ZDR），灵活的数据保留策略配置。',
        highlight1: '默认不保留请求内容',
        highlight2: '灵活的数据保留策略配置',
        highlight3: '数据最小化原则贯彻',
        highlight4: 'Aggregate/Audit/Detail三级模式'
      },
      hipaa: {
        title: 'HIPAA 医疗合规',
        description: '医疗行业专属合规框架，支持医疗建议人工监督和患者数据保护。',
        highlight1: '医疗建议人工监督机制',
        highlight2: '患者数据严格保护',
        highlight3: '730天合规数据保留期限',
        highlight4: '完整的审计追踪能力'
      },
      credentials: {
        title: '一站式合规凭证',
        description: '五种合规凭证自动生成，一键导出合规报告。',
        highlight1: 'GDPR合规凭证',
        highlight2: 'EU AI Act评估报告',
        highlight3: '零数据保留声明',
        highlight4: '数据处理协议（DPA）'
      },
      templates: {
        title: '行业合规模板',
        description: '医疗、金融、教育、电商四大行业模板，预置规则开箱即用。',
        highlight1: '医疗健康行业合规模板',
        highlight2: '金融服务行业合规模板',
        highlight3: '教育培训行业合规模板',
        highlight4: '电子商务行业合规模板'
      },
      risk: {
        title: '风险分析与监控',
        description: '实时风险标签监控，异常行为检测，确保AI应用安全合规。',
        highlight1: '实时风险标签监控',
        highlight2: '异常行为检测',
        highlight3: '合规政策违规预警',
        highlight4: '审计日志追溯'
      }
    },
    certificates: {
      title: '合规凭证',
      description: '一站式合规凭证管理，自动生成、一键导出',
      gdpr: 'GDPR合规凭证',
      euai: 'EU AI Act评估报告',
      zdr: '零数据保留声明',
      dpa: '数据处理协议',
      hipaa: 'HIPAA医疗合规声明',
      security: '安全认证凭证'
    },
    templates: {
      title: '行业合规模板',
      description: '预置合规策略，开箱即用',
      apply: '应用模板',
      healthcare: '医疗健康',
      healthcareDesc: '医疗建议人工监督、HIPAA合规、患者数据保护、数据保留730天',
      finance: '金融服务',
      financeDesc: '信用评分人工监督、审计留痕、反欺诈、数据保留365天',
      education: '教育培训',
      educationDesc: '学习评估人工监督、未成年人数据保护、数据保留180天',
      ecommerce: '电子商务',
      ecommerceDesc: '推荐引擎用户画像告知、数据保留90天'
    },
    cta: {
      title: '开启AI合规之旅',
      description: '立即体验ThreeRouter企业级AI治理与合规解决方案，让AI合规变得简单。',
      primary: '立即体验',
      secondary: '登录控制台'
    },
    samples: {
      title: '报告样本',
      description: '查看我们的合规报告样本，了解ThreeRouter如何帮助企业满足法规要求',
      euAiAct: {
        title: 'EU AI Act评估报告样本',
        description: '查看完整的AI系统角色定位和高风险评估'
      },
      gdprRopa: {
        title: 'GDPR ROPA报告样本',
        description: '查看完整的处理活动记录和合法依据声明'
      }
    },
    footer: '© 2026 ThreeRouter. All rights reserved.'
  },
  whitepaper: {
    type: '白皮书',
    title: 'ThreeRouter AI治理与合规白皮书',
    subtitle: '深入了解ThreeRouter如何帮助企业应对EU AI Act、GDPR等全球AI监管法规，构建安全、合规的AI应用架构。',
    downloadBtn: '下载PDF白皮书',
    download: '白皮书下载功能即将上线，请关注后续更新。',
    back: '返回合规页面',
    toc: '目录',
    footer: '© 2026 ThreeRouter. All rights reserved.',
    chapters: {
      intro: {
        title: '第一章：AI监管时代的挑战',
        description: '随着EU AI Act、GDPR等法规的出台，企业面临前所未有的AI合规挑战。',
        section1: '1.1 全球AI监管趋势',
        content1: '欧盟AI法案（EU AI Act）是全球第一部全面的AI监管法规，将AI系统分为四个风险等级，要求高风险AI系统必须经过严格的合规评估。与此同时，GDPR对数据处理活动的要求也越来越严格，企业需要完善的数据处理记录（ROPA）来满足审计要求。',
        section2: '1.2 企业面临的合规挑战',
        content2: '企业在AI应用过程中面临多重合规挑战：如何确定AI系统的法律角色？如何评估AI系统的风险等级？如何确保跨境数据传输合规？如何管理第三方AI模型供应商的合规责任？',
        section3: '1.3 ThreeRouter的解决方案',
        content3: 'ThreeRouter提供完整的AI治理与合规框架，帮助企业应对上述挑战。通过内置的合规能力，企业可以快速完成EU AI Act评估、GDPR ROPA编制、数据处理协议生成等任务。'
      },
      euai: {
        title: '第二章：EU AI Act合规框架',
        description: '深入了解ThreeRouter的EU AI Act合规评估体系。',
        section1: '2.1 AI系统角色定位',
        content1: 'ThreeRouter帮助企业明确AI系统在EU AI Act下的法律角色。通过专业的角色映射分析，企业可以确定自己是AI系统提供者、部署者还是基础设施服务提供者，避免法律角色混淆带来的合规风险。',
        section2: '2.2 高风险场景评估',
        content2: '基于EU AI Act Annex III的高风险场景清单，ThreeRouter帮助企业评估AI系统是否涉及高风险应用场景。评估结果考虑AI系统的用途、数据处理方式、决策影响等多个维度。',
        section3: '2.3 GPAI下游集成商合规',
        content3: '对于使用第三方通用目的AI模型的企业，ThreeRouter提供下游集成商合规框架，明确企业与模型提供者之间的责任边界，确保符合EU AI Act对GPAI的特殊要求。'
      },
      gdpr: {
        title: '第三章：GDPR合规实践',
        description: '完整的GDPR合规解决方案，从数据处理记录到数据主体权利。',
        section1: '3.1 处理活动记录（ROPA）',
        content1: 'ThreeRouter帮助企业编制符合GDPR Art.30要求的处理活动记录。记录包括处理活动描述、合法依据、数据类别、接收方、跨境传输机制、保留期限等关键信息。',
        section2: '3.2 合法依据管理',
        content2: '确保所有数据处理活动都有合法依据是GDPR合规的核心。ThreeRouter支持多种合法依据的管理，包括合同履行、合法利益、法律义务等，并帮助企业评估合法依据的适用性。',
        section3: '3.3 数据主体权利支持',
        content3: 'ThreeRouter提供数据主体权利支持机制，帮助企业响应数据访问、删除、可携性、限制处理、反对等请求，确保符合GDPR对数据主体权利的保障要求。'
      },
      architecture: {
        title: '第四章：零数据保留架构',
        description: '了解ThreeRouter独特的零数据保留架构如何保障数据安全。',
        section1: '4.1 ZDR模式原理',
        content1: '零数据保留（Zero Data Retention）是ThreeRouter的核心安全架构。默认情况下，API请求和响应内容不会被保留，只有聚合后的使用指标会被记录，最大限度保护用户数据隐私。',
        section2: '4.2 灵活的保留策略',
        content2: 'ThreeRouter支持三种数据保留模式：Aggregate Only（仅聚合）、Audit（审计模式）和Detail（详细记录）。企业可以根据自身合规需求灵活配置数据保留策略。',
        section3: '4.3 数据最小化原则',
        content3: '数据最小化是GDPR的基本原则之一。ThreeRouter在设计上贯彻数据最小化原则，只收集和处理实现服务目的所必需的数据，避免过度收集个人信息。'
      },
      implementation: {
        title: '第五章：合规实施指南',
        description: '如何在实际业务中实施AI合规。',
        section1: '5.1 合规评估流程',
        content1: 'ThreeRouter提供完整的合规评估流程：首先进行AI系统角色和风险评估，然后选择适用的行业合规模板，配置数据保留策略，最后生成合规凭证和报告。',
        section2: '5.2 行业合规模板应用',
        content2: '针对医疗、金融、教育、电商等不同行业，ThreeRouter提供预置的合规模板，包含行业特定的合规要求和最佳实践，帮助企业快速满足行业合规标准。',
        section3: '5.3 持续合规监控',
        content3: '合规不是一次性任务，而是持续的过程。ThreeRouter提供实时风险监控和合规状态追踪，帮助企业及时发现合规风险，确保AI应用始终符合监管要求。'
      }
    }
  },
  enterprise: {
    nav: {
      enterprise: '企业服务',
      authority: '权威推荐',
      compare: '对比',
      cases: '成功案例',
      cta: '马上开通团队账号',
      login: '登录',
      backHome: '返回首页',
    },
    hero: {
      badge: '企业级 AI API 管理',
      title: '企业快速接入全球最强大模型',
      subtitle: '团队统一管理 · 安全 · 合规 · 可信',
      roleLabel: '选择您的角色，获取专属方案',
      roleDecisionTitle: '我是决策者',
      roleDecisionDesc: 'CEO / CTO / 技术负责人 / 采购经理',
      roleDecisionFocus: '关注安全合规、成本控制、团队管理',
      roleDecisionTag: '企业采购方案',
      roleEmployeeTitle: '我是公司员工',
      roleEmployeeDesc: '开发者 / 产品经理 / 设计师',
      roleEmployeeFocus: '关注用好 AI 模型、团队协作效率',
      roleEmployeeTag: '推荐公司接入',
      decisionTitle: '作为决策者',
      decisionDesc: '您需要安全、合规、成本可控的 API 接入方案。ThreeRouter 提供企业级安全认证、团队配额管理、用量透明可控。',
      employeeTitle: '作为团队员工',
      employeeDesc: '推荐公司接入 ThreeRouter，一人申请，全团队受益。统一 API 网关，一行代码切换模型。',
      ctaPrimary: '马上开通团队账号 →',
      ctaSecondary: '对比普通中转站',
      ctaEmployee: '推荐给公司 →',
      statCustomers: '企业客户信赖',
      statRequests: '累计 API 调用量',
      statUptime: '服务可用性 SLA',
      statLeaks: '数据泄露事故',
      statCustomersValue: '5,000+',
      statRequestsValue: '50亿+',
      statUptimeValue: '99.9%',
      statLeaksValue: '0'
    },
    authority: {
      label: '业界前瞻',
      title: 'AI 时代，企业需要统一的 Token 管理平台',
      subtitle: '未来，公司用 Token 买时间。员工配备 Token 额度，就像配备电脑和办公位一样，将成为企业统一招聘福利。',
      quote1: '「过去，公司用人买时间；未来，公司用 Token 买时间。」',
      quote1Desc: 'AI 时代的价值创造公式正在被重写。最贵的不是写代码的人，而是用好 AI 工具的人。',
      quote2: '「Token 正在成为 AI 原生企业的货币。」',
      quote2Desc: '企业为员工配备统一的 AI Token 账户，比让员工各自购买更安全、更便宜、更可控。',
      quote3: '「AI 原生公司只需要 2 个人：CEO 和 CTO。」',
      quote3Desc: '每个员工都将拥有 AI 助理，而 Token 就是他们的生产力燃料。企业批量采购 Token 是必然选择。',
      cta: '了解企业方案 →'
    },
    pain: {
      label: '市场现状',
      title: '那些「中转站」不会告诉你的三件事',
      subtitle: 'Token API 市场鱼龙混杂。大量中转站以极低价格吸引用户，背后却是数据窃取、模型偷换、随时跑路的重重风险。',
      item1Title: '数据裸奔，中转站全量抄送',
      item1Desc: '你的每一次对话、每一段业务数据，都经过中转站服务器。他们可以全量存储、分析、转卖。今年已有数十起中转站数据泄露事件。',
      item2Title: '模型偷换，你付 pro版本 得 flash版本',
      item2Desc: '这是中转站行业公开的秘密：前端显示 pro版本，后端换成 flash版本 甚至开源模型。实测最高 68% 的请求被降级。',
      item3Title: '随时跑路，连发票都没有',
      item3Desc: '大量中转站是个人运营，没有企业资质、没有合规发票、没有 SLA。今年以来已有超过 30 个知名中转站突然关停。'
    },
    solution: {
      label: 'ThreeRouter 解决方案',
      title: '让 CEO 安心，让 CTO 省心',
      subtitle: '精准触达企业决策链条上的每个角色，一套方案满足全团队需求。',
      card1Badge: '安全合规零风险',
      card1Title: 'Token 就是生产力，投入即回报',
      card1Desc: 'API 接入不是成本，而是投资。让团队用上好模型，研发效率提升 10 倍。',
      card1Points: [
        '用量可视化，成本透明可控',
        '对公转账、合规发票',
        '批量采购比个人购买节省 30%+'
      ],
      card1Btn: '预约方案演示',
      card2Badge: '成本透明省 30%',
      card2Title: '安全合规，数据零风险',
      card2Desc: '选择合规、透明、可审计的 API 接入路径，让安全风险不再困扰。',
      card2Points: [
        '数据不存储、不中转、不出境',
        '模型透明，承诺不偷换',
        '企业级 SLA 保障（99.9%）'
      ],
      card2Btn: '查看技术文档',
      card2BtnLink: '/help-cn.html',
      card3Badge: '统一 API 提效',
      card3Title: '一人申请，全团队用上顶级模型',
      card3Desc: '推荐公司接入，统一 API 网关，一行代码切换模型，开发效率起飞。',
      card3Points: [
        '统一 API 接口，兼容 OpenAI 格式',
        '公司统一付费，不用个人充值',
        'DeepSeek、Kimi、Glm、Minimax 一网打尽'
      ],
      card3Btn: '推荐给公司'
    },
    team: {
      label: '团队管理',
      title: '智能团队管理，让 API 调用尽在掌控',
      subtitle: '部门、消费者、密钥、用量、成本 —— 一站式管理，全景可视。',
      feature1Title: '层级化部门管理',
      feature1Desc: '通过清晰的树形结构组织团队，创建嵌套部门、分配部门编码，一目了然地查看汇报关系。',
      feature1Points: [
        '树形部门可视化展示',
        '部门编码自定义，便于成本归集',
        '部门级别的用量与成本追踪'
      ],
      feature2Title: '消费者全生命周期管理',
      feature2Desc: '从创建到停用，管理每一个 API 消费者。将消费者分配到部门，设置类型和角色，实时追踪其活动状态。',
      feature2Points: [
        '快速完成信息录入表单',
        '类型优先选择，清晰分类管理',
        '完整档案支持：姓名、邮箱、电话、职位、部门'
      ],
      feature3Title: '智能关联的 API 密钥管理',
      feature3Desc: '将 API 密钥关联到特定部门和消费者，实现精细化访问控制。选择部门后，系统自动筛选该部门下的可选消费者。',
      feature3Points: [
        '部门感知的消费者智能联动',
        '安全的密钥生成与轮换机制',
        '单密钥级别的用量与成本归因'
      ],
      feature4Title: '全维度团队数据分析',
      feature4Desc: '用丰富的多维度分析面板，做出数据驱动的决策。支持按天/按小时粒度切换，灵活选择任意时间段。',
      feature4Points: [
        '使用趋势：按天/按小时粒度切换',
        '平台分布：直观展示各 AI 平台资源消耗',
        '消费排行：精准定位高频消费者',
        '成本洞察：总请求数、Token、成本一目了然'
      ],
      feature5Title: '角色化团队成员管理',
      feature5Desc: '精细化的角色分配，掌控每个成员的权限边界。添加管理员、邀请成员、管理待处理邀请。',
      feature5Points: [
        '管理员与普通成员角色体系',
        '邀请流程支持，待处理状态实时追踪',
        '活跃成员统计一览无余'
      ],
      feature6Title: '实时成本优化',
      feature6Desc: '每一次请求都被记录，每一个 Token 都被计算，每一笔成本都被归因。依托 1 分钟级聚合引擎，分析面板反映实时用量。',
      feature6Points: [
        '1 分钟数据聚合，近实时洞察',
        '按消费者、按部门、按团队的多维成本拆解',
        '历史趋势分析，助力预算规划'
      ]
    },
    tokenManagement: {
      label: '精细化管理',
      title: '企业 Token 使用精细化管理',
      subtitle: '每一个 Token 都被追踪、每一次调用都被记录、每一笔成本都被归因。从部门到个人、从模型到请求，全链路可视化管控。',
      metrics: [
        {
          value: '1min',
          label: '数据聚合粒度',
          desc: '近实时用量洞察'
        },
        {
          value: '6维度',
          label: '成本拆解维度',
          desc: '团队/部门/消费者/模型/密钥/请求'
        },
        {
          value: '100%',
          label: '成本归因覆盖率',
          desc: '每笔费用可追溯'
        },
        {
          value: '0漏报',
          label: '用量数据完整性',
          desc: '不遗漏任何调用'
        }
      ],
      features: [
        {
          icon: '🎯',
          iconBg: 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300',
          title: '多维度用量追踪',
          desc: '按模型、按消费者、按部门、按密钥、按时间维度，全链路追踪 Token 消耗。',
          points: [
            '模型级 Token 消耗排行',
            '消费者级别的精细用量归因',
            '部门维度的成本聚合视图',
            '时间序列趋势分析（天/小时）'
          ]
        },
        {
          icon: '💰',
          iconBg: 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300',
          title: '智能成本管控',
          desc: '预算设定、配额分配、超限告警，让每一分 AI 投入都有据可查、有度可控。',
          points: [
            '按部门/消费者设置 Token 配额上限',
            '预算超限自动告警通知',
            '成本趋势预测与预算规划',
            '实时余额与消耗速率监控'
          ]
        },
        {
          icon: '🔑',
          iconBg: 'bg-purple-100 text-purple-600 dark:bg-purple-900/30 dark:text-purple-300',
          title: '密钥级精细管控',
          desc: '每个 API 密钥独立计费、独立配额、独立审计，实现真正的最小粒度管控。',
          points: [
            '单密钥用量与成本独立核算',
            '密钥级别配额与速率限制',
            '密钥生命周期管理（创建/轮换/停用）',
            '密钥关联部门与消费者追溯'
          ]
        },
        {
          icon: '📊',
          iconBg: 'bg-cyan-100 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-300',
          title: '实时数据看板',
          desc: '1 分钟级数据聚合引擎，近实时反映团队 AI 使用全貌，决策不再滞后。',
          points: [
            '近实时用量数据刷新',
            '多维度交叉分析面板',
            '可导出的定制化报表',
            'API 调用趋势可视化图表'
          ]
        },
        {
          icon: '🛡️',
          iconBg: 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300',
          title: '合规审计追踪',
          desc: '完整记录每一次 API 调用，满足企业内部审计与行业合规要求。',
          points: [
            '全量调用日志可追溯',
            '数据处理活动记录（GDPR 兼容）',
            '审计报告自动生成',
            '操作行为全程留痕'
          ]
        },
        {
          icon: '⚡',
          iconBg: 'bg-rose-100 text-rose-600 dark:bg-rose-900/30 dark:text-rose-300',
          title: '智能优化建议',
          desc: '基于历史用量数据，自动识别优化机会，帮助企业持续降低 AI 成本。',
          points: [
            '高频低效调用自动识别',
            '模型选择优化建议',
            '闲置资源回收提醒',
            '成本节省机会实时推送'
          ]
        }
      ],
      processTitle: 'Token 精细化管理四步法',
      processSteps: [
        {
          title: '设定预算配额',
          desc: '按部门和消费者分配 Token 预算与使用配额'
        },
        {
          title: '实时监控追踪',
          desc: '1 分钟级数据聚合，全维度监控用量与成本'
        },
        {
          title: '告警与优化',
          desc: '超限告警、异常检测、自动推送优化建议'
        },
        {
          title: '审计与复盘',
          desc: '生成审计报告，复盘 AI 投入产出比'
        }
      ]
    },
    teamWorkflow: {
      label: '快速上手',
      title: '创建团队，统一管理',
      subtitle: '三步创建团队，统一管理成员权限与用量配额。',
      step1Title: '创建团队空间',
      step1Desc: '管理员快速创建团队空间，配置成员权限与用量配额。支持按角色精细化管控，确保安全合规。',
      step1Points: [
        '一键创建团队空间',
        '精细化成员权限管理',
        '用量配额灵活配置'
      ],
      step2Title: '邀请加入团队',
      step2Desc: '管理员一键生成邀请链接，团队成员点击即可加入。无需繁琐注册，统一团队计费与审计。',
      step2Points: [
        '一键生成邀请链接',
        '团队成员即点即用',
        '统一计费与审计'
      ],
      step3Title: '成员与配额管控',
      step3Desc: '添加或移除团队成员，按角色分配访问权限。实时监控用量，设置差异化配额，合理分配资源。',
      step3Points: [
        '灵活成员管理',
        '实时用量监控',
        '差异化配额分配'
      ]
    },
    compare: {
      label: '透明对比',
      title: 'ThreeRouter vs 普通中转站',
      subtitle: '一张表看懂差距。选 Token API 服务，不只是看价格。',
      dimension: '对比维度',
      us: 'ThreeRouter',
      them: '普通中转站',
      rows: {
        entity: {
          label: '运营实体',
          us: '正规公司运营',
          them: '多无工商注册，个人小作坊'
        },
        security: {
          label: '数据安全',
          us: '不存储对话数据，端到端加密',
          them: '可完整记录并转卖数据'
        },
        transparency: {
          label: '模型透明性',
          us: '透明运行，源头可追溯',
          them: '可偷换降级模型，无法验证'
        },
        sla: {
          label: 'SLA 保障',
          us: '99.9% 可用性，有 SLA 承诺',
          them: '无 SLA，随时跑路'
        },
        invoice: {
          label: '发票支持',
          us: '合规发票，对公转账',
          them: '大多无发票能力'
        },
        support: {
          label: '售后支持',
          us: '专业团队，专人对接',
          them: '基本为零'
        },
        price: {
          label: '价格',
          us: '市场合理价位',
          them: '极低价甚至 1 折'
        }
      }
    },
    employeeApply: {
      label: '自下而上',
      title: '一键向老板申请，让公司为你买单',
      subtitle: '花公司的钱，用最好的 AI 模型。填好信息，一键生成申请理由，提交给老板审批。',
      cardTitle: '一步生成申请文案',
      generateBtn: '生成申请文案 →',
      benefitsTitle: '老板为什么应该批？',
      benefits: [
        {
          title: '老板为什么应该批？',
          desc: 'ThreeRouter 是官方出品，安全合规、品牌可靠。花的钱是团队效率投资，不是个人消费。'
        },
        {
          title: '用了能提升多少？',
          desc: '接入 AI API 后，代码生成、调试、文档编写效率可提升 5-10 倍。实习生也能产出高质量代码。'
        },
        {
          title: '完全合规，财务无忧',
          desc: '支持对公转账、可开合规发票。不偷偷摸摸，财务审计也完全 OK。'
        },
        {
          title: '一个人申请，全团队受益',
          desc: '一旦公司开通，整个技术团队都能用上最好的 AI 模型。你一个人推动，带动团队升级。'
        }
      ],
      applicationLabel: '申请文案',
      copyBtn: '复制',
      generatedLabel: '申请文案已生成',
      copySuccess: '已复制',
      applicationTemplate: {
        greeting: '老板，您好：',
        intro: '我建议公司为技术团队开通 ThreeRouter 企业账号，统一管理 AI API 调用。理由如下：',
        reason1: '1. 安全可靠：ThreeRouter 提供企业级安全与合规，数据不存储、不中转，支持对公开票。',
        reason2: '2. 成本可控：统一采购比个人购买更便宜，用量可视化，预算清晰。',
        reason3: '3. 效率提升：统一 API 网关兼容 OpenAI 格式，一行代码切换 GPT-5.5、Claude、DeepSeek 等模型。',
        reason4: '4. 团队管理：支持部门、消费者、密钥、用量、成本一站式管理，方便后续扩展。',
        closing: '请审批，谢谢！'
      }
    },
    cases: {
      label: '成功案例',
      title: '先行的企业已经在用了',
      subtitle: '越来越多的技术团队选择 ThreeRouter。',
      case1Company: '某跨国公司中国团队内部使用',
      case1Title: '统一 API 网关，50 人团队效率提升 5 倍',
      case1Desc: '接入 ThreeRouter 后，全技术团队统一使用一套 API 网关，不再各自对接不同模型。运维成本降低 70%，模型调用成本降低 40%。',
      case2Company: '武汉##科技技术团队',
      case2Title: '从个人中转站迁移到 ThreeRouter，数据安全终于放心了',
      case2Desc: '之前团队有人使用个人中转站服务，CTO 担心数据安全问题。接入 ThreeRouter 后，所有 API 调用纳入企业级安全体系。',
      case3Company: '##云 · 技术部',
      case1Initial: '视',
      case2Initial: '武',
      case3Initial: '数',
      case3Title: '稳定可靠，支撑日调用量百万级',
      case3Desc: '日均 API 调用量超过 120 万次，ThreeRouter 始终保持 99.95% 可用性。专业团队 7×24 小时响应。'
    },
    cta: {
      label: '开始使用',
      title: '今天就让团队用上安全可靠的 AI API',
      subtitle: '选择最适合你的方式开始',
      primary: '马上开通团队账号 →',
      secondary: '我是员工，推荐给公司 →'
    },
    faq: {
      eyebrow: 'FAQ',
      title: '企业服务常见问题',
      subtitle: '关于企业接入、安全合规、Token 管理和团队协作的常见问题解答',
      q1: '企业接入流程是怎样的？需要多长时间？',
      a1: '标准企业接入可在 1 个工作日内完成。流程包括：开通企业账号 → 配置团队结构和部门配额 → 分发 API Key → 开始调用。我们提供全程技术支持，确保平滑迁移。',
      q2: '如何保障企业数据安全与合规？',
      a2: '采用端到端加密传输（TLS 1.3），不存储用户请求数据和响应内容，不用于模型训练。平台支持完整的操作审计日志，满足 GDPR、欧盟 AI 法案等合规要求。上游资源均来自 AWS、GCP、Azure 等官方云厂商，采购链路合法可审计。',
      q3: 'Token 精细化管理支持哪些维度？',
      a3: '支持按部门、团队、项目、API Key 四个维度进行 Token 配额管控。每个维度可独立设置用量上限、告警阈值和成本归因。管理员可实时查看各维度 Token 消耗明细，支持按时间段导出报表。',
      q4: '是否提供 SLA 服务等级协议？',
      a4: '企业客户提供 99.9% 可用性 SLA 保障。平台采用多供应商路由和故障转移机制，确保服务高可用。如未达到 SLA 承诺，将按协议约定提供额度补偿。',
      q5: '是否支持私有化部署或专属实例？',
      a5: '支持。企业可选择云上专属实例（ Dedicated Cloud）或完全私有化部署（On-Premise）。专属实例提供独立资源池和网络隔离，私有化部署支持部署在企业自有服务器或私有云环境中。',
      q6: '如何与现有系统和技术栈集成？',
      a6: '完全兼容 OpenAI API 格式，只需替换 base URL 和 API Key 即可接入。支持所有主流 SDK（Python、Node.js、Go、Java）、Agent 框架（LangChain、AutoGen、CrewAI）和工作流工具。迁移成本仅需一行代码修改。',
      q7: '团队权限管理支持哪些角色和粒度？',
      a7: '支持管理员、部门负责人、开发者、只读用户四种内置角色，并支持自定义角色。权限粒度覆盖模型访问、API Key 管理、用量查看、配额设置、账单管理等操作。支持按部门和项目进行权限隔离。',
      q8: '是否提供企业专属技术支持？',
      a8: '提供。企业客户享有专属技术支持通道，包括 7x24 小时工单响应、企业微信群支持、定期架构巡检和用量优化建议。大型企业可指定专属客户成功经理（CSM），提供一对一服务。'
    },
    footer: {
      brandDesc: 'ThreeRouter 是多模型 API 统一管理平台，帮助团队快速接入全球最强大模型，提供安全、合规、稳定的企业级服务。',
      copyright: '© 2026 ThreeRouter. All rights reserved.',
      slogan: '安全 · 合规 · 可信'
    }
  }
}
