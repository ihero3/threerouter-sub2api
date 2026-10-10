/**
 * en fork 差异字典（生成文件，可手动维护）。
 *
 * - 仅包含本 fork 独有的 key，以及与上游 locales/en/ 不同的文案；
 *   合并时本文件的值优先（见 locales/en.ts 的 deepMerge）。
 * - 修改 fork 文案：直接编辑本文件。
 * - 同步上游新文案：更新 locales/en/ 下的模块化文件。
 * - 重新生成本文件：node scripts/gen-fork-locales.mjs
 *   （注意：会以当前 locales/en.ts 为基准覆盖本文件，手改内容会丢失，
 *    只有在明确要重新提取差异时才运行。）
 */
export default {
  home: {
    viewOnGithub: 'GitHub',
    viewDocs: 'Docs',
    enterprise: 'Enterprise',
    models: 'Models',
    getStarted: 'Get $10 ≈ 330M Free Tokens',
    heroDescription: 'Access DeepSeek, Qwen, Kimi, GLM and more with a single API key — at 1/33 the cost of GPT-5.5',
    tags: {
      subscriptionToApi: 'OpenAI-Compatible',
      stickySession: 'US Data Centers'
    },
    hero: {
      slogan: 'DeepSeek, Qwen, Kimi, GLM — the lowest price is just 1/33 of GPT',
      title: 'Three Router',
      subtitle: 'Top-tier Open Source LLMs with Unbeatable Value,Enterprise-Grade LLM',
      cta: 'Get $10 ≈ 330M Free Tokens →',
      codeHint: 'Change one URL, keep everything else.',
      coreDiff: 'US Local Deployment + up to 97% Cost Cut',
      priceAdvantage: 'Pay 1/33. Never full price.',
      priceReasons: 'US-East & US-West · Direct from Cloud Providers',
      tags: {
        discount: 'DeepSeek V4-Pro from $0.42/M',
        deployment: 'US Local Deployment',
        freeTokens: 'New Users Get $10 ≈ 330M Free Tokens'
      },
      costChart: {
        eyebrow: 'US vs Open Source model cost',
        title: 'More production requests from the same $5 budget',
        usRoute: 'Deployment',
        maxSaving: 'Max saving',
        note: 'Compared by $/M input tokens: US-local entry keeps latency low while Open Source frontier models compress cost dramatically.'
      },
      pricingSection: {
        tableHeader: {
          title: 'Price Comparison',
          model: 'Model',
          capability: 'threerouter ($/M input)',
          price: 'OpenAI ($/M input)',
          relative: 'You Save'
        },
        models: {
          gpt55: {
            name: 'GPT-5.5',
            capability: 'OpenAI latest flagship'
          },
          claudeOpus48: {
            name: 'Claude Opus 4.8',
            capability: 'Anthropic latest flagship, SWE-bench Pro 69.2%'
          },
          glm51: {
            name: 'GLM-5.2',
            capability: 'Zhipu flagship, SWE-bench Top Tier'
          },
          kimiK26: {
            name: 'Kimi K2.6',
            capability: 'Open-source SOTA, SWE-Bench Pro 58.6%'
          },
          deepseekV4Flash: {
            name: 'DeepSeek V4-Flash',
            capability: 'Best value, MoE architecture'
          },
          deepseekV4Pro: {
            name: 'DeepSeek V4-Pro',
            capability: 'Flagship reasoning, Codeforces #1'
          },
          minimaxM3: {
            name: 'MiniMax-M3',
            capability: '1M context, frontier coding, native multimodality'
          },
          qwen37Max: {
            name: 'Qwen3.8-Max',
            capability: 'Alibaba agent flagship, 1M context, top coding, complex workflows & multi-framework generalization'
          },
          seedance20: {
            name: 'Seedance-2.0',
            capability: 'ByteDance video generation, cinematic output'
          }
        },
        baseline: 'Baseline',
        discount: 'Save {percent}%',
        directProcurement: 'Direct Procurement from Cloud Providers & AI Platforms',
        directProcurementDesc: 'Sourced from AWS, GCP, Azure, and Alibaba Cloud directly. Not resold, not re-routed through gray channels. Get an audit trail your security team can actually read.',
        enterpriseBadge: 'ENTERPRISE COMPLIANT GATEWAY',
        benefits: {
          title: 'Enterprise Benefits',
          compliant: 'Compliant with industry regulations',
          stable: '99.99% service availability',
          traceable: 'Full audit trail & certification',
          secure: 'Enterprise-grade security'
        }
      }
    },
    painPoints: {
      items: {
        expensive: {
          title: 'Paying 33x for GPT',
          desc: 'GPT-5.5 costs $5/M. DeepSeek V4-Pro costs $0.42/M. Same intelligence, 1/33 the price.'
        },
        complex: {
          title: 'Juggling API Keys',
          desc: 'Managing separate accounts for DeepSeek, Qwen, Kimi, and GLM across different platforms'
        },
        unstable: {
          title: 'Latency from Open Source Regions',
          desc: 'Most Open Source LLM APIs route through their origin regions. We route through US-East and US-West.'
        },
        noControl: {
          title: 'No Usage Visibility',
          desc: 'Can\'t track token spend per model, per team, or per API key'
        }
      }
    },
    solutions: {
      title: 'One API. Every Open Source LLM.',
      subtitle: 'Three steps to slash your LLM bill'
    },
    features: {
      unifiedGateway: 'One API Key',
      unifiedGatewayDesc: 'Call DeepSeek, Qwen, Kimi, GLM, MiniMax, and Seedance through a single API key. OpenAI-compatible.',
      multiAccount: 'US-Local Routing',
      multiAccountDesc: 'Requests hit US-East / US-West nodes. P99 latency under 200ms — even when the model is in Shanghai.',
      balanceQuotaDesc: 'Usage-based billing with quota limits. Full visibility into token consumption per model and per team.'
    },
    easyrouterAdvantages: {
      eyebrow: 'Why threerouter',
      title: 'Why threerouter, not the other gateways?',
      subtitle: 'US-local infrastructure. Direct cloud procurement. OpenAI drop-in replacement.',
      ultraFast: {
        title: 'US-Local, not Beijing-Local',
        desc: 'Most Open Source LLM APIs route through their origin regions. We don\'t. Your requests hit our US-East / US-West nodes and get sub-200ms P99 — even when the model is DeepSeek V4 in its origin region.'
      },
      reliable: {
        title: 'Compliance You Can Show Your Lawyer',
        desc: 'Sourced from AWS, GCP, Azure, and Alibaba Cloud directly. Not resold, not re-routed through gray channels. Get an audit trail your security team can actually read.'
      },
      standardApi: {
        title: 'OpenAI-Compatible, No Rewrite',
        desc: 'Drop-in replacement. Your existing code, your existing tools, your existing workflows. Change one URL, keep everything.'
      },
      cheap: {
        title: 'Super Affordable',
        desc: 'Our prices are only 1/33 of flagship providers, helping you reduce AI costs by up to 97%'
      }
    },
    easyrouterFaq: {
      eyebrow: 'FAQ',
      title: 'Frequently Asked Questions',
      subtitle: 'Everything you need to know about service, billing, and integration.',
      tabs: {
        service: 'About Service',
        billing: 'Pricing & Billing',
        integration: 'Integration & Usage'
      },
      service: {
        q1: 'Are you a proxy or reseller?',
        a1: 'Neither. We are an enterprise-grade AI API gateway. Our upstream providers are official top-tier cloud vendors such as AWS, GCP, and Azure. For open-source models, our sources are directly from model providers or globally recognized inference services. The pipeline is legal, transparent, and auditable.',
        q2: 'Where do your model resources come from?',
        a2: 'Our resources come from official cloud providers, AI platforms, model vendors, and globally recognized inference services. This ensures stable access, compliant procurement, and traceable delivery.',
        q3: 'Do you store user request data?',
        a3: 'No. The gateway is designed for secure pass-through routing. We do not use customer prompts or responses for model training, and we minimize data retention for operational security.',
        q4: 'Which models are supported?',
        a4: 'We cover DeepSeek, Qwen, Kimi, MiniMax, GLM, and Seedance video generation models. More coming soon.'
      },
      billing: {
        q1: 'Is this a monthly subscription?',
        a1: 'No. Credits are deducted based on actual model usage. You pay for what you consume — no fixed monthly fee for unused capacity.',
        q2: 'How is usage billed?',
        a2: 'Text models are billed by token usage. Image, video, speech, and other multimodal models follow their provider-specific billing units (image count, video seconds, audio duration).',
        q3: 'Are prices transparent?',
        a3: 'Yes. Pricing is displayed by model and billing unit. Estimate costs before use and monitor actual consumption in your dashboard.',
        q4: 'Can teams share credits?',
        a4: 'Yes. Team and enterprise usage can be managed through shared quota pools, permission controls, and usage analytics.'
      },
      integration: {
        q1: 'Is it compatible with the OpenAI API?',
        a1: 'Yes. The gateway is fully compatible with the OpenAI API format. Replace the base URL and API key, and you\'re ready to go.',
        q2: 'How much migration work is required?',
        a2: 'One line of code. SDKs, agents, and workflow tools continue using familiar request formats — just point them at threerouter.com/v1.',
        q3: 'Do you support failover and smart routing?',
        a3: 'Yes. We use multi-provider routing, load balancing, and failover strategies to improve availability and reduce disruptions.',
        q4: 'Can it be used in production systems?',
        a4: 'Yes. The gateway is designed for production with standardized APIs, high availability routing, usage observability, and enterprise compliance.'
      }
    },
    comparison: {
      headers: {
        official: 'Direct API Subscriptions',
        us: 'threerouter'
      }
    },
    providers: {
      loginToView: 'Login to View'
    },
    cta: {
      title: 'Stop paying 33x for the same intelligence.',
      description: 'Get $10 ≈ 330M free tokens. No credit card required.',
      button: 'Get $10 ≈ 330M Free Tokens — No credit card →'
    },
    reviews: {
      title: 'Built for developers and enterprises',
      subtitle: 'Real metrics from production',
      review1: {
        text: 'API calls in the last 24 hours',
        name: '1.2M+',
        role: 'requests served'
      },
      review2: {
        text: 'P99 latency from US-East',
        name: '187ms',
        role: 'average response time'
      },
      review3: {
        text: 'Uptime over the last 90 days',
        name: '99.99%',
        role: 'service availability'
      },
      review4: {
        text: 'Customer data retention',
        name: 'Zero',
        role: 'privacy by design'
      },
      review5: {
        text: 'Community',
        name: '1.2K ★',
        role: 'stars'
      },
      review6: {
        text: 'Performance Rankings',
        name: '#1 Product',
        role: 'of the Day'
      },
      review7: {
        text: 'G2 user rating',
        name: '4.8 ★',
        role: 'verified reviews'
      },
      review8: {
        text: 'Models available',
        name: '15+',
        role: 'and growing'
      }
    },
    testimonials: {
      eyebrow: 'Customer Stories',
      title: 'What developers say',
      subtitle: 'Real feedback from production workloads',
      t1: {
        name: 'Alex Chen',
        role: 'SaaS Founder',
        quote: 'After switching to Three Router, our AI inference bill dropped from $4,000 to $120 per month. API-compatible, migration took 10 minutes.'
      },
      t2: {
        name: 'Maria Rodriguez',
        role: 'Full-stack Engineer',
        quote: 'DeepSeek V4-Pro through the US local endpoint has low latency and costs 1/12 of OpenAI. It is our default for production now.'
      },
      t3: {
        name: 'Sam Liu',
        role: 'CTO, AI Startup',
        quote: 'Moving all production LLM calls to Three Router cut our costs by over 90% while staying rock-solid stable.'
      }
    },
    compliancePromo: {
      eyebrow: 'AI Governance & Compliance',
      title: 'Compliance for Enterprise AI Applications',
      description: 'ThreeRouter includes global regulatory compliance frameworks such as EU AI Act and GDPR, automatically generating compliance credentials and processing records.',
      button: 'Explore Compliance',
      badge: 'Enterprise Compliance',
      features: {
        euai: {
          title: 'EU AI Act Compliance Assessment',
          desc: 'AI system role mapping and legal classification, high-risk scenario evaluation based on Annex III.'
        },
        gdpr: {
          title: 'GDPR Art.30 ROPA',
          desc: 'Complete records of processing activities, compliant legal basis declarations, EU SCC cross-border transfer assurance.'
        },
        zdr: {
          title: 'Zero Data Retention',
          desc: 'Default no request content retention, flexible data retention policy configuration, data minimization principles.'
        },
        creds: {
          title: 'One-Stop Compliance Credentials',
          desc: 'Five types of compliance credentials auto-generated, one-click export compliance reports including GDPR, EU AI Act.'
        },
        templates: {
          title: 'Industry Compliance Templates',
          desc: 'Four industry templates: healthcare, finance, education, e-commerce. Pre-configured rules ready to use.'
        },
        risk: {
          title: 'Risk Analysis & Monitoring',
          desc: 'Real-time risk tag monitoring, anomaly detection, compliance policy violation alerts, audit log tracing.'
        }
      },
      certsLabel: 'Compliance Certifications:'
    },
    footer: {
      documentation: 'Documentation',
      advantage: 'Our Advantages',
      contact: 'Contact Us',
      legalNotice: 'For legal notices, please contact us via email first'
    }
  },
  common: {
    expandAll: 'Expand all',
    collapseAll: 'Collapse all'
  },
  validation: {
    required: '{field} is required',
    maxLength: '{field} must be at most {max} characters',
    minLength: '{field} must be at least {min} characters'
  },
  nav: {
    models: 'Model Plaza',
    teamManagement: 'Team Management',
    teamMembers: 'Team Members',
    departments: 'Departments',
    consumers: 'Consumers',
    teamAnalytics: 'Team Analytics',
    teamSettings: 'Team Settings',
    tickets: 'My Tickets',
    ticketManagement: 'Ticket Management',
    governance: 'AI Governance',
    batchImage: 'Batch Image'
  },
  tickets: {
    fields: {
      id: 'ID',
      contact: 'Contact',
      title: 'Title',
      category: 'Category',
      priority: 'Priority',
      status: 'Status',
      content: 'Description',
      updatedAt: 'Updated At'
    },
    pricing: {
      input: 'Input',
      output: 'Output',
      approx: 'Approx'
    },
    categories: {
      account: 'Account',
      billing: 'Billing',
      api: 'API Issue',
      model: 'Model / Channel',
      other: 'Other'
    },
    priorities: {
      low: 'Low',
      normal: 'Normal',
      high: 'High',
      urgent: 'Urgent'
    },
    statuses: {
      open: 'Open',
      pending: 'Waiting for User',
      answered: 'Answered',
      closed: 'Closed'
    },
    filters: {
      allStatuses: 'All statuses',
      allCategories: 'All categories'
    },
    author: {
      user: 'User',
      admin: 'Support'
    },
    placeholders: {
      contact: 'Email,Tel , or another contact method',
      title: 'Briefly describe your issue',
      content: 'Describe the issue, API endpoint, error message, or relevant context in detail'
    },
    actions: {
      new: 'Submit Ticket',
      submit: 'Submit Ticket',
      submitting: 'Submitting...',
      reply: 'Send Reply',
      backToList: 'Back to Tickets'
    },
    new: {
      title: 'Submit Ticket',
      description: 'Submit a ticket for account, billing, API, model, or channel issues.',
      loggedHint: 'You are signed in. This ticket will be linked to your account.',
      guestHint: 'You are not signed in. Please leave a valid contact method.'
    },
    my: {
      title: 'My Tickets',
      description: 'View your submitted tickets, status updates, and support replies.',
      empty: 'No tickets yet.'
    },
    detail: {
      title: 'Ticket Detail',
      reply: 'Reply',
      closedHint: 'This ticket is closed and cannot be replied to.'
    },
    messages: {
      created: 'Ticket submitted',
      createFailed: 'Failed to submit ticket',
      loadFailed: 'Failed to load tickets',
      replied: 'Reply sent',
      replyFailed: 'Failed to send reply'
    }
  },
  auth: {
    verificationCodeHint: 'Enter the code sent to your email',
    invalidCode: 'Please enter a valid 6-character code'
  },
  keys: {
    departmentLabel: 'Department',
    consumerLabel: 'Consumer (Employee)',
    selectDepartment: 'Select a department',
    selectConsumer: 'Select a consumer',
    noDepartment: 'No department',
    noDepartments: 'No departments yet',
    noConsumer: 'No consumer'
  },
  admin: {
    models: {
      title: 'Model Plaza',
      description: 'Manage and configure available AI models',
      copy: 'Copy model name',
      hint: 'Access More Models via API Key',
      loginHint: 'Log in to view discounted prices',
      loginAction: 'Log In',
      status: {
        available: 'Available'
      },
      pricing: {
        input: 'Input',
        output: 'Output',
        approx: 'Approx'
      },
      categories: {
        text: 'Text',
        image: 'Image',
        audio: 'Audio',
        multimodal: 'Multimodal'
      }
    },
    team: {
      members: {
        title: 'Team Members',
        subtitle: 'Manage team member information, roles and permissions',
        description: 'Manage team members',
        addMember: 'Add Member',
        totalMembers: 'Total Members',
        activeMembers: 'Active Members',
        admins: 'Admins',
        pending: 'Pending',
        searchPlaceholder: 'Search member name or email',
        allRoles: 'All Roles',
        allStatus: 'All Status',
        status: {
          active: 'Active',
          pending: 'Pending',
          inactive: 'Inactive'
        },
        columns: {
          name: 'Name',
          role: 'Role',
          department: 'Department',
          status: 'Status',
          joinedAt: 'Joined At',
          actions: 'Actions'
        },
        roles: {
          owner: 'Owner',
          admin: 'Admin',
          manager: 'Manager',
          member: 'Member',
          viewer: 'Viewer'
        },
        noMembers: 'No Members',
        addFirstMember: 'Add your first team member to get started',
        deleteMember: 'Delete Member',
        deleteConfirm: 'Are you sure you want to delete member {name}? This action cannot be undone.',
        editNotImplemented: 'Edit feature not yet implemented',
        memberEnabled: 'Member enabled',
        memberDisabled: 'Member disabled',
        memberDeleted: 'Member deleted',
        inviteMember: 'Invite Member',
        emailLabel: 'Email',
        emailPlaceholder: 'Enter member email',
        roleLabel: 'Role',
        sendInvite: 'Send Invite',
        inviting: 'Sending...',
        emailRequired: 'Email is required',
        emailInvalid: 'Invalid email format',
        editRole: 'Edit Role',
        roleDescriptions: {
          owner: 'Full team control and management',
          admin: 'Manage members, settings, and billing',
          manager: 'Manage departments and team operations',
          member: 'Use team resources and view analytics',
          viewer: 'View team data only'
        },
        fetchFailed: 'Failed to fetch team members',
        inviteSuccess: 'Member invited successfully',
        inviteFailed: 'Failed to invite member',
        inviteNotImplemented: 'Invite feature not yet implemented',
        removeMember: 'Remove Member',
        removeConfirm: 'Are you sure you want to remove member {name}? This action cannot be undone.',
        roleUpdated: 'Member role updated successfully'
      },
      departments: {
        title: 'Departments',
        subtitle: 'Manage department structure and staffing',
        description: 'Manage department structure',
        addDepartment: 'Add Department',
        members: 'Members',
        managers: 'Managers',
        head: 'Head',
        deleteDepartment: 'Delete Department',
        deleteConfirm: 'Are you sure you want to delete department {name}? This action cannot be undone.',
        editNotImplemented: 'Edit feature not yet implemented',
        departmentDeleted: 'Department deleted',
        name: 'Department Name',
        namePlaceholder: 'Enter department name',
        code: 'Department Code',
        codePlaceholder: 'Enter department code',
        costCenterCode: 'Department Code',
        costCenterCodePlaceholder: 'Enter department code',
        descriptionLabel: 'Description',
        descriptionPlaceholder: 'Enter department description',
        parentDepartment: 'Parent Department',
        selectParent: 'Select parent department',
        noParent: 'No parent (top level)',
        createDepartment: 'Create Department',
        editDepartment: 'Edit Department',
        addChild: 'Add Child Department',
        noDepartments: 'No departments yet',
        noResults: 'No departments found',
        noResultsDescription: 'Try adjusting your search or filters',
        addFirstDepartment: 'Get started by creating your first department',
        fetchFailed: 'Failed to fetch departments',
        createSuccess: 'Department created successfully',
        createFailed: 'Failed to create department',
        editSuccess: 'Department updated successfully',
        updateFailed: 'Failed to update department',
        deleteFailed: 'Failed to delete department',
        searchPlaceholder: 'Search department name or description',
        allStatus: 'All Status',
        createdAt: 'Created At',
        treeView: 'Tree View',
        gridView: 'Grid View',
        reorderSuccess: 'Department hierarchy updated',
        reorderFailed: 'Failed to update department hierarchy',
        reorderCycle: 'Cannot move a department under its own sub-department',
        status: {
          active: 'Active',
          inactive: 'Inactive'
        }
      },
      consumers: {
        title: 'Consumers',
        subtitle: 'Manage consumer accounts and usage',
        description: 'Manage consumers',
        addConsumer: 'Add Consumer',
        totalConsumers: 'Total Consumers',
        activeConsumers: 'Active Consumers',
        newThisMonth: 'New This Month',
        inactive: 'Inactive',
        searchPlaceholder: 'Search consumer name or email',
        allTypes: 'All Types',
        allStatus: 'All Status',
        status: {
          active: 'Active',
          inactive: 'Inactive'
        },
        types: {
          enterprise: 'Enterprise',
          premium: 'Premium',
          standard: 'Standard'
        },
        columns: {
          name: 'Name',
          type: 'Type',
          company: 'Company',
          usage: 'Usage',
          status: 'Status',
          createdAt: 'Created At',
          actions: 'Actions'
        },
        noConsumers: 'No Consumers',
        addFirstConsumer: 'Add your first consumer to get started',
        deleteConsumer: 'Delete Consumer',
        deleteConfirm: 'Are you sure you want to delete consumer {name}? This action cannot be undone.',
        editNotImplemented: 'Edit feature not yet implemented',
        detailsNotImplemented: 'Detail view not yet implemented',
        consumerDeleted: 'Consumer deleted',
        fetchFailed: 'Failed to fetch consumers',
        createSuccess: 'Consumer created successfully',
        createNotImplemented: 'Create feature not yet implemented',
        createFailed: 'Failed to create consumer',
        editConsumer: 'Edit Consumer',
        editSuccess: 'Consumer updated successfully',
        editFailed: 'Failed to update consumer',
        deleteFailed: 'Failed to delete consumer',
        detailsTitle: 'Consumer Details',
        detailsFailed: 'Failed to fetch consumer details',
        namePlaceholder: 'Enter name',
        nameRequired: 'Name is required',
        descriptionLabel: 'Description',
        descriptionPlaceholder: 'Enter description',
        noDescription: 'No description',
        fields: {
          id: 'ID',
          teamId: 'Team ID',
          department: 'Department',
          updatedAt: 'Updated At'
        },
        offboardTitle: 'Offboard Consumer',
        offboardWarning: 'This action will permanently disable the consumer',
        offboardWarningDetail: 'Are you sure you want to offboard {name}?',
        affectedKeys: 'Affected API Keys',
        affectedKeysCount: '{count} API key(s) will be disabled',
        noKeysFound: 'No API keys found for this consumer',
        consequences: 'Consequences',
        consequenceKeysDisabled: 'All API keys will be disabled',
        consequenceConsumerInactive: 'Consumer status will be set to inactive',
        consequenceIrreversible: 'This action cannot be undone',
        confirmLabel: 'Type {name} to confirm',
        offboarding: 'Offboarding...',
        confirmOffboard: 'Confirm Offboard',
        confirmRequired: 'Confirmation is required',
        confirmMismatch: 'Confirmation text does not match'
      },
      analytics: {
        title: 'Team Analytics',
        subtitle: 'View team usage and performance metrics',
        description: 'View team statistics',
        totalRequests: 'Total Requests',
        totalCost: 'Total Cost',
        usageTrend: 'Usage Trend',
        platformDistribution: 'Platform Distribution',
        topConsumers: 'Top Consumers',
        last7Days: 'Last 7 Days',
        last30Days: 'Last 30 Days',
        last90Days: 'Last 90 Days',
        today: 'Today',
        last3Days: 'Last 3 Days',
        last15Days: 'Last 15 Days',
        thisMonth: 'This Month',
        lastMonth: 'Last Month',
        custom: 'Custom',
        granularityDay: 'By Day',
        granularityHour: 'By Hour',
        viewConsumerDetails: 'View Consumer Details',
        detailsNotImplemented: 'Detail view not yet implemented',
        fetchFailed: 'Failed to fetch analytics',
        columns: {
          consumer: 'Consumer',
          requests: 'Requests',
          cost: 'Cost',
          actions: 'Actions'
        }
      },
      settings: {
        title: 'Team Settings',
        subtitle: 'Manage team info, members, billing, and security settings',
        tabs: {
          general: 'General',
          members: 'Members',
          billing: 'Billing',
          danger: 'Danger Zone'
        },
        general: {
          title: 'General Info',
          description: 'Set team name, description, avatar, timezone, and language',
          teamName: 'Team Name',
          teamNamePlaceholder: 'Enter team name',
          descriptionLabel: 'Description',
          descriptionPlaceholder: 'Enter team description',
          teamAvatar: 'Team Avatar',
          uploadAvatar: 'Upload Avatar',
          avatarHint: 'JPG/PNG supported, recommended 200x200',
          timezone: 'Timezone',
          language: 'Language',
          lang: {
            zh: '简体中文',
            en: 'English'
          },
          saveSuccess: 'General settings saved'
        },
        members: {
          inviteTitle: 'Invite Member',
          inviteDescription: 'Invite a new member by email',
          emailPlaceholder: 'Enter member email',
          namePlaceholder: 'Enter member name (optional)',
          inviteButton: 'Invite',
          inviteSuccess: 'Invitation sent',
          listTitle: 'Member List',
          columns: {
            name: 'Name',
            email: 'Email',
            role: 'Role',
            status: 'Status',
            joinedAt: 'Joined',
            actions: 'Actions'
          },
          roles: {
            owner: 'Owner',
            member: 'Member',
            manager: 'Manager',
            admin: 'Admin',
            viewer: 'Viewer'
          },
          roleUpdated: '{name}\'s role updated',
          memberEnabled: '{name} enabled',
          memberDisabled: '{name} disabled',
          memberRemoved: '{name} removed',
          emailRequired: 'Please enter an email address'
        },
        billing: {
          title: 'Billing Settings',
          description: 'Configure payment method, billing address, and invoice settings',
          paymentMethod: 'Payment Method',
          methods: {
            creditCard: 'Credit Card',
            alipay: 'Alipay',
            wechatPay: 'WeChat Pay',
            bankTransfer: 'Bank Transfer'
          },
          billingAddress: 'Billing Address',
          addressPlaceholder: 'Enter billing address',
          invoiceSettings: 'Invoice Settings',
          autoInvoice: 'Auto-generate invoices',
          invoiceEmail: 'Email invoices',
          invoiceEmailAddress: 'Invoice Email',
          invoiceEmailPlaceholder: 'Enter invoice email',
          saveSuccess: 'Billing settings saved'
        },
        danger: {
          title: 'Danger Zone',
          description: 'These actions are irreversible. Please proceed with caution.',
          transferOwnership: 'Transfer Ownership',
          transferOwnershipDesc: 'Transfer team ownership to another member',
          transferButton: 'Transfer',
          transferDialogTitle: 'Confirm Transfer Ownership',
          transferDialogMessage: 'Are you sure you want to transfer team ownership? This cannot be undone.',
          transferSuccess: 'Ownership transferred',
          deleteTeam: 'Delete Team',
          deleteTeamDesc: 'Deleting the team will erase all data. This cannot be recovered.',
          deleteButton: 'Delete Team',
          deleteDialogTitle: 'Confirm Delete Team',
          deleteDialogMessage: 'Are you sure you want to delete the team? This will erase all data and cannot be undone!',
          deleteSuccess: 'Team deleted'
        }
      }
    },
    tickets: {
      title: 'Ticket Management',
      description: 'View, reply to, and resolve user support tickets.',
      searchPlaceholder: 'Search title or contact',
      empty: 'No tickets yet.',
      selectHint: 'Select a ticket on the left to view details.',
      replyPlaceholder: 'Enter support reply',
      statusUpdated: 'Ticket status updated',
      statusUpdateFailed: 'Failed to update ticket status'
    },
    backup: {
      imageStorage: {
        title: 'Image Storage',
        description: 'Configure S3-compatible object storage used to host user-uploaded images (backups reuse this configuration by default).',
        enabled: 'Enable image storage',
        reuseBackupS3: 'Reuse backup S3 configuration',
        bucketInherited: 'Inherits from backup S3 configuration',
        prefix: 'Key Prefix',
        publicBaseUrl: 'Public Base URL',
        publicBaseUrlPlaceholder: 'Optional. Public CDN/origin URL prefix used to render uploaded images in the UI.',
        presignExpiryHours: 'Presign Expiry (hours)',
        saved: 'Image storage configuration saved'
      },
      actions: {
        downloadFailed: 'Download failed',
        downloadParts: 'Backup Parts',
        downloadPartsHint: 'Backups above 5GB are split into multiple parts. Download all parts and concatenate them in order to restore.'
      }
    },
    users: {
      searchUsers: 'Search by email, username, notes, or API key...',
      soraStorageQuota: 'Sora Storage Quota',
      soraStorageQuotaHint: 'In GB, 0 means use group or system default quota',
      bulkLimits: {
        title: 'Bulk Edit Limits',
        selectUser: 'Select a user: {email}',
        selectedCount: '{count} user selected | {count} users selected',
        apply: 'Apply',
        enableConcurrency: 'Toggle concurrency',
        enableRPMLimit: 'Toggle RPM limit',
        nonNegativeInteger: 'Please enter a non-negative integer (0 for unlimited)',
        selectionLimit: 'Bulk edit supports up to {max} users per batch',
        concurrencyValue: 'Set concurrency to {value}',
        rpmValue: 'Set RPM to {value}',
        rpmUnlimitedValue: 'Set RPM to unlimited',
        confirm: 'Apply changes to {count} user? | Apply changes to {count} users?',
        success: 'Updated {count} user successfully | Updated {count} users successfully',
        failed: 'Bulk edit failed'
      }
    },
    groups: {
      columnSettings: 'Column settings',
      duplicateSuccess: 'Group "{name}" duplicated',
      usageYesterday: 'Yesterday Usage'
    },
    channels: {
      form: {
        codexImageGenerationBridgeHint: 'When enabled, Codex /responses text requests in OpenAI groups may be automatically given the image_generation tool. Keep off unless the routed accounts support image generation.'
      }
    },
    governance: {
      title: 'AI Governance & Compliance',
      description: 'Governance for EU AI Act and GDPR: audit trails, risk tagging, assessment reports and data subject rights',
      refresh: 'Refresh',
      loadFailed: 'Failed to load',
      status: {
        primaryRole: 'Primary Role',
        riskTier: 'Risk Tier',
        capabilities: 'Enabled Capabilities'
      },
      capability: {
        risk_tagging: 'Risk Tagging',
        audit_logging: 'Audit Logging',
        gdpr_erasure: 'GDPR Erasure',
        gdpr_data_export: 'Data Export',
        consent_management: 'Consent Mgmt',
        eu_ai_act_report: 'EU AI Act Report'
      },
      tabs: {
        audit: 'Audit Logs',
        risk: 'Risk Analysis',
        euAiAct: 'EU AI Act Assessment',
        ropa: 'GDPR Processing Record',
        erasure: 'GDPR Erasure Requests',
        templates: 'Industry Templates',
        rules: 'Content Moderation Rules',
        jurisdiction: 'Jurisdiction Mapping',
        dpa: 'DPA Agreement',
        credentials: 'Compliance Credentials'
      },
      audit: {
        complianceType: 'Compliance Type',
        complianceTypePlaceholder: 'e.g. risk_assessment',
        subjectType: 'Subject Type',
        subjectTypePlaceholder: 'e.g. user',
        subjectId: 'Subject ID',
        details: 'Details',
        operator: 'Operator',
        createdAt: 'Time',
        search: 'Search',
        reset: 'Reset'
      },
      risk: {
        modelTags: 'Model Attribute Tags',
        riskTags: 'Compliance Risk Tags'
      },
      euAiAct: {
        export: 'Export Report',
        exportSuccess: 'Report exported',
        exportFailed: 'Export failed',
        empty: 'No assessment report yet, click refresh to generate'
      },
      ropa: {
        empty: 'No data processing record yet, click refresh to generate'
      },
      erasure: {
        userId: 'User ID',
        requestType: 'Request Type',
        status: 'Status',
        statusAll: 'All',
        statusPending: 'Pending',
        statusApproved: 'Approved',
        statusRejected: 'Rejected',
        statusCompleted: 'Completed',
        requestedAt: 'Requested At',
        actions: 'Actions',
        approve: 'Approve',
        reject: 'Reject',
        approveTitle: 'Approve Erasure Request',
        rejectTitle: 'Reject Erasure Request',
        confirmHint: 'Confirm processing erasure request #{id}?',
        reason: 'Rejection Reason',
        reasonPlaceholder: 'Please enter a rejection reason',
        reasonRequired: 'A reason is required when rejecting',
        processSuccess: 'Processed successfully',
        processFailed: 'Processing failed'
      },
      templates: {
        empty: 'No industry compliance templates yet',
        create: 'Create Custom Template',
        createTitle: 'Create Custom Compliance Template',
        apply: 'Apply',
        active: 'Active',
        rules: 'Policy Rules',
        applySuccess: 'Template applied; compliance audit recorded',
        applyFailed: 'Failed to apply template',
        createSuccess: 'Template created',
        createFailed: 'Failed to create template',
        requiredFields: 'Template code and industry are required',
        invalidRules: 'Policy rules must be a valid JSON array',
        fieldCode: 'Template Code',
        fieldIndustry: 'Industry',
        fieldDescription: 'Description',
        fieldRules: 'Policy Rules (JSON array)',
        fieldRiskTags: 'Risk Tags',
        riskTagsPlaceholder: 'Separate multiple tags with commas'
      },
      rules: {
        strategy: 'Strategy Combination',
        strategySuccess: 'Strategy updated',
        strategyFailed: 'Failed to update strategy',
        create: 'New Rule',
        createTitle: 'New Moderation Rule',
        editTitle: 'Edit Moderation Rule',
        ruleId: 'Rule Code',
        ruleName: 'Rule Name',
        ruleType: 'Rule Type',
        rulePattern: 'Match Pattern',
        patternPlaceholder: 'KEYWORD: keyword; REGEX: regular expression; PATTERN: * wildcard',
        threshold: 'Threshold/Weight',
        priority: 'Priority',
        riskCategory: 'Risk Category',
        action: 'Action',
        enabled: 'Enabled',
        disabled: 'Disabled',
        enabledLabel: 'Enable this rule',
        requiredFields: 'Rule code, name and match pattern are required',
        saveSuccess: 'Rule saved',
        saveFailed: 'Failed to save rule',
        deleteSuccess: 'Rule deleted',
        deleteFailed: 'Failed to delete rule'
      },
      jurisdiction: {
        region: 'Company Jurisdiction',
        industry: 'Industry',
        industryPlaceholder: 'e.g. healthcare / finance / ecommerce',
        serviceType: 'Service Type',
        map: 'Generate Mapping',
        mapFailed: 'Failed to generate jurisdiction mapping',
        riskLevel: 'Risk Level',
        regulations: 'Applicable Regulations',
        measures: 'Compliance Measures',
        actions: 'Recommended Actions',
        fieldHelp: 'Field Help',
        fieldHelpRegion: 'The business region where you operate, used to identify applicable data protection regulations (e.g., GDPR, China PIPL)',
        fieldHelpIndustry: 'Your industry sector, which affects risk assessment levels and compliance requirements (e.g., healthcare, finance, education have special requirements)',
        fieldHelpServiceType: 'The type of AI service you provide, helping the system identify specific compliance obligations and assessment criteria. Options: AI Chatbot (AI chatbot service), AI Analysis (AI data analysis service), AI Recommendation (AI recommendation service)'
      },
      dpa: {
        title: 'DPA Compliance Statement',
        controllerName: 'Data Controller Name',
        controllerNamePlaceholder: 'Enter data controller company name',
        controllerContact: 'Data Controller Contact',
        controllerContactPlaceholder: 'Enter contact email or phone',
        generate: 'Generate DPA',
        requiredFields: 'Please fill all required fields',
        generateSuccess: 'DPA Compliance Statement generated successfully',
        generateFailed: 'Failed to generate DPA Compliance Statement'
      },
      credentials: {
        status: 'Status',
        create: 'Create Credential',
        credentialId: 'Credential ID',
        type: 'Type',
        issuer: 'Issuer',
        validUntil: 'Valid Until',
        createdAt: 'Created At',
        revoke: 'Revoke',
        revokeSuccess: 'Credential revoked',
        revokeFailed: 'Failed to revoke credential',
        activate: 'Activate',
        activateSuccess: 'Credential activated',
        activateFailed: 'Failed to activate credential',
        deleteSuccess: 'Credential deleted',
        deleteFailed: 'Failed to delete credential',
        loadFailed: 'Failed to load credentials',
        requiredFields: 'Please fill all required fields',
        createSuccess: 'Credential created',
        createFailed: 'Failed to create credential'
      }
    },
    dataRights: {
      title: 'Data Subject Rights',
      description: 'Manage your data subject rights: export personal data, request data deletion, and manage consent records.',
      export: {
        title: 'Data Export',
        description: 'Under GDPR Article 20, you can export your personal data.',
        button: 'Export Data',
        processing: 'Processing...',
        success: 'Export request submitted, Export ID: {id}',
        successMessage: 'Data export request submitted',
        error: 'Failed to export data'
      },
      erasure: {
        title: 'Data Erasure',
        description: 'Under GDPR Article 17, you can request deletion of your personal data.',
        reasonLabel: 'Reason',
        reasonPlaceholder: 'Please explain why you want your data deleted...',
        confirmLabel: 'Confirmation Text',
        confirmPlaceholder: 'Type "DELETE MY DATA" to confirm',
        confirmHint: 'Type DELETE MY DATA to confirm deletion',
        submit: 'Submit Erasure Request',
        submitting: 'Submitting...',
        success: 'Erasure request submitted, Request ID: {id}',
        successMessage: 'Data erasure request submitted',
        error: 'Failed to submit erasure request',
        history: 'Erasure Request History',
        historyDescription: 'View the status of your submitted data erasure requests.',
        noRequests: 'No erasure requests',
        request: 'Erasure Request',
        requestType: 'Request Type',
        reason: 'Reason',
        status: {
          pending: 'Pending',
          approved: 'Approved',
          rejected: 'Rejected',
          completed: 'Completed'
        }
      },
      consent: {
        title: 'Consent Records',
        description: 'View and manage your data processing consent records.',
        empty: 'No consent records',
        version: 'Version',
        status: 'Status',
        granted: 'Granted',
        revoked: 'Revoked',
        grantedAt: 'Granted At',
        loadError: 'Failed to load consent records',
        updateSuccess: 'Consent record updated',
        updateError: 'Failed to update consent record',
        types: {
          terms_of_service: {
            label: 'Terms of Service',
            description: 'I have read and agree to the Terms of Service (required)'
          },
          gdpr_data_processing: {
            label: 'GDPR Data Processing Agreement',
            description: 'I consent to the processing of my personal data in accordance with the GDPR Data Processing Agreement (required)'
          },
          detailed_logging: {
            label: 'Detailed Logging',
            description: 'Allow the system to record detailed request and response logs for auditing and troubleshooting'
          },
          cross_border_transfer: {
            label: 'Cross-border Data Transfer',
            description: 'Allow data transfer between different countries/regions to provide services'
          },
          marketing: {
            label: 'Marketing',
            description: 'Receive product updates, promotions, and other marketing communications'
          },
          model_training: {
            label: 'Model Training Data',
            description: 'Allow your data to be used for AI model improvement and optimization'
          }
        }
      }
    },
    compliance: {
      title: 'Account Compliance Config',
      description: 'Configure your Account-level AI governance and compliance policies, including industry templates, ZDR mode, compliance frameworks, and content moderation rules.',
      template: {
        title: 'Industry Template',
        description: 'Choose a compliance template suitable for your industry to quickly apply predefined compliance policies.',
        apply: 'Apply Template',
        current: 'Current Template',
        industries: {
          ecommerce: {
            label: 'E-commerce',
            description: 'E-commerce industry compliance template: Recommendation engine user profiling notice, data retention 90 days.'
          },
          finance: {
            label: 'Financial Services',
            description: 'Financial industry compliance template: Credit scoring human oversight, audit trail, anti-fraud, data retention 365 days.'
          },
          healthcare: {
            label: 'Healthcare',
            description: 'Healthcare industry compliance template: Medical advice human oversight, HIPAA compliance, patient data protection, data retention 730 days.'
          },
          education: {
            label: 'Education',
            description: 'Education industry compliance template: Learning assessment human oversight, minor data protection, data retention 180 days.'
          }
        }
      },
      zdr: {
        title: 'ZDR Settings',
        description: 'Configure Zero Data Retention (ZDR) mode to control data retention policies.',
        mode: 'ZDR Mode',
        aggregate_only: 'Aggregate Only',
        audit: 'Audit',
        retention_days: 'Detail Log Retention Days'
      },
      frameworks: {
        title: 'Compliance Frameworks',
        description: 'Select applicable compliance frameworks to ensure your Account meets relevant regulatory requirements.',
        gdpr: 'GDPR',
        eu_ai_act: 'EU AI Act',
        hipaa: 'HIPAA'
      },
      moderation: {
        title: 'Content Moderation Policy',
        description: 'Manage content moderation rules, enable or disable specific moderation policies.',
        enabled: 'Enabled'
      },
      customRules: {
        title: 'Custom Moderation Rules',
        description: 'Create your own content moderation rules to supplement the system defaults.',
        create: 'New Rule',
        createTitle: 'Create Custom Rule',
        editTitle: 'Edit Custom Rule',
        edit: 'Edit',
        delete: 'Delete',
        update: 'Save',
        enabled: 'Enabled',
        disabled: 'Disabled',
        enableRule: 'Enable Rule',
        empty: 'No custom rules yet',
        ruleName: 'Rule Name',
        ruleNamePlaceholder: 'e.g. Internal Banned Words',
        ruleType: 'Rule Type',
        rulePattern: 'Match Pattern',
        patternPlaceholder: 'e.g. banned|sensitive (use | for multiple REGEX keywords)',
        action: 'Action',
        riskCategory: 'Risk Category',
        riskCategoryPlaceholder: 'e.g. Custom',
        createSuccess: 'Custom rule created successfully',
        updateSuccess: 'Custom rule updated successfully',
        deleteSuccess: 'Custom rule deleted successfully',
        deleteConfirm: 'Are you sure you want to delete this custom rule?'
      },
      status: {
        title: 'Config Status'
      },
      jurisdiction: {
        title: 'Jurisdiction Mapping',
        description: 'Automatically map applicable compliance regulations and requirements based on your business region, industry, and service type.',
        region: 'Company Region',
        industry: 'Industry',
        industryPlaceholder: 'e.g. Healthcare, Financial Services',
        serviceType: 'Service Type',
        map: 'Start Mapping',
        save: 'Save Configuration',
        applyRules: 'Apply Compliance Rules',
        saved: 'Configuration saved',
        appliedRules: 'Applied Rules',
        saveSuccess: 'Configuration saved successfully',
        riskLevel: 'Risk Level',
        regulations: 'Applicable Regulations',
        measures: 'Required Measures',
        actions: 'Recommended Actions'
      },
      dpa: {
        title: 'DPA Compliance Statement',
        description: 'Generate DPA Compliance Statement to demonstrate GDPR Art.28 compliance elements. This is a compliance declaration, not a legally binding contract.',
        controllerName: 'Data Controller Name',
        controllerNamePlaceholder: 'Enter your company name',
        controllerContact: 'Data Controller Contact',
        controllerContactPlaceholder: 'Enter contact email or phone',
        generate: 'Generate DPA',
        success: 'DPA Compliance Statement generated successfully, file downloaded.'
      },
      credentials: {
        title: 'Compliance Credentials',
        description: 'View your compliance credentials, including certifications, audit reports, etc.',
        empty: 'No compliance credentials yet',
        validFrom: 'Valid From',
        validUntil: 'Valid Until',
        scope: 'Scope',
        issuerType: 'Issuer Type',
        credentialTypes: {
          GDPR_COMPLIANCE: 'GDPR Compliance Credential',
          EU_AI_ACT_ASSESSMENT: 'EU AI Act Assessment',
          ZERO_DATA_RETENTION: 'Zero Data Retention',
          DPA_COMPLIANCE: 'Data Processing Agreement (DPA)',
          SECURITY_CERTIFICATION: 'Security Certification'
        },
        issuerTypes: {
          SELF_ASSERTION: 'Self-Assertion',
          THIRD_PARTY: 'Third-Party'
        },
        metadata: {
          title: 'Details',
          compliance_basis: 'Compliance Basis',
          data_processing_record: 'Processing Record',
          dpo_contact: 'DPO Contact',
          data_retention: 'Data Retention',
          assessment_date: 'Assessment Date',
          risk_category: 'Risk Category',
          human_in_the_loop: 'Human in the Loop',
          model_training: 'Model Training',
          prompt_storage: 'Prompt Storage',
          policy_version: 'Policy Version',
          request_content: 'Request Content',
          technical_logs: 'Technical Logs',
          security_logs: 'Security Logs',
          backup_policy: 'Backup Policy',
          dpa_version: 'DPA Version',
          scc_compliant: 'SCC Compliant',
          subprocessor_approval_required: 'Subprocessor Approval Required',
          data_subject_rights_supported: 'Supported Data Subject Rights',
          encryption_at_rest: 'Encryption at Rest',
          transport_security: 'Transport Security',
          access_control: 'Access Control',
          audit_logging: 'Audit Logging',
          security_audits: 'Security Audits'
        }
      }
    },
    subscriptions: {
      daysRemaining: 'days remaining',
      revokeConfirm: 'Are you sure you want to revoke the subscription for \'{user}\'? This action cannot be undone.',
      guide: {
        actions: {
          revokeDesc: 'Immediately terminate the subscription (irreversible)'
        }
      }
    },
    accounts: {
      bulkEdit: {
        baseUrlNotice: 'Applies to API Key accounts only; leave empty to keep existing value'
      },
      openai: {
        wsModeDesc: 'Only applies to the current OpenAI account type.',
        codexImageGenerationBridge: 'Codex image-generation bridge',
        codexImageGenerationBridgeDesc: 'Account policy takes precedence over channel and global settings. Only controls whether Codex requests through the /responses text endpoint receive the image_generation tool; standalone image-generation endpoints are unaffected.',
        codexImageGenerationBridgeInherit: 'Follow channel',
        codexImageGenerationBridgeInheritDesc: 'Do not write an account override; use the channel or global policy.',
        codexImageGenerationBridgeEnabled: 'Force on',
        codexImageGenerationBridgeEnabledDesc: 'Allow image tool injection for Codex /responses requests.',
        codexImageGenerationBridgeDisabled: 'Force off',
        codexImageGenerationBridgeDisabledDesc: 'Block image tool injection for Codex /responses requests.',
        codexImageGenerationBridgeBadgeInherit: 'Channel policy',
        codexImageGenerationBridgeBadgeEnabled: 'Account on',
        codexImageGenerationBridgeBadgeDisabled: 'Account off'
      },
      oauth: {
        openai: {
          codexSessionAuth: 'Codex JSON / AT Batch Input',
          codexSessionDesc: 'Paste Codex JSON or an accessToken. Accounts use the step 1 settings.',
          codexSessionInputLabel: 'Codex JSON or accessToken',
          codexSessionPlaceholder: 'Multiple lines supported, one token or JSON per line',
          codexSessionHint: 'sessionToken will not be saved as refresh_token. Without refresh_token, the account expires with the accessToken expiry; import is rejected if the expiry cannot be parsed and step 1 has no expiration.',
          codexSessionEmpty: 'Please enter Codex JSON or accessToken'
        }
      },
      imageTestHint: 'When an image model is selected, this test sends a real image-generation request and previews the returned image below.',
      openaiQuotaReset: {
        resetSuccess: 'Reset {windows} window(s)'
      }
    },
    proxies: {
      batchInputPlaceholder: 'Enter one proxy per line in the following formats:\nsocks5://user:pass 192.168.1.1:1080\nhttp://192.168.1.1:8080\nhttps://user:pass&#64;proxy.example.com:443',
      batchInputHint: 'Supports http, https, socks5 protocols. Format: protocol://[user:pass&#64;]host:port',
      fallbackMode: 'Failure fallback'
    },
    usage: {
      department: 'Department',
      consumer: 'Consumer',
      departmentPlaceholder: 'Filter by Department Name',
      consumerPlaceholder: 'Filter by Consumer Name'
    },
    ops: {
      errorDetail: {
        attemptedKeyPrefix: 'Attempted Key Prefix',
        deletedKeyOwner: 'Deleted Key Owner'
      },
      settings: {
        ignoreInvalidApiKeyErrors: 'Ignore invalid API key errors',
        ignoreInvalidApiKeyErrorsHint: 'When enabled, invalid or missing API key errors (INVALID_API_KEY, API_KEY_REQUIRED) will not be written to the error log.'
      }
    },
    settings: {
      features: {
        channelMonitor: {
          description: 'Periodically probe configured channels and surface availability / latency to users. Turning it off stops the scheduler and returns an empty list on the user page.',
          enabledHint: 'Disabling stops background checks; existing history is preserved.',
          defaultIntervalHint: 'Pre-fills the interval when creating a new monitor; each monitor can override it. Range 15 – 3600.'
        }
      },
      registration: {
        emailSuffixWhitelistHint: 'Only email addresses from the specified domains can register (for example, &#64;qq.com, &#64;gmail.com, *.edu.cn)',
        emailSuffixWhitelistPlaceholder: '&#64;example.com, *.edu.cn'
      },
      apiKeyAcl: {
        description: 'Choose which client IP is used by API Key allowlists and denylists',
        trustForwardedIpHint: 'Disabled by default. Enable only when the origin is reachable only through Cloudflare or Nginx reverse proxy. When enabled, API Key IP allowlists and denylists use CF-Connecting-IP, X-Real-IP, or X-Forwarded-For, matching the request IP shown in usage records.'
      },
      gatewayForwarding: {
        claudeOAuthSystemPromptBlocksPlaceholder: "Leave empty to use the built-in 3 blocks. Supports an array or {'{'}\"blocks\": [...]{'}'}.",
        claudeOAuthSystemPromptBlocksHint: 'Each block is saved as JSON with enabled, type, text, and optional cache_control. {billing_header} stays dynamic per request; the Claude Code identity and expansion prompts can be edited directly or restored from presets.'
      },
      soraClient: {
        title: 'Sora Client',
        description: 'Control whether to show the Sora client entry in the sidebar',
        enabled: 'Enable Sora Client',
        enabledHint: 'When enabled, the Sora entry will be shown in the sidebar for users to access Sora features'
      },
      smtp: {
        usernamePlaceholder: 'your-email&#64;gmail.com',
        fromEmailPlaceholder: 'noreply&#64;example.com'
      },
      testEmail: {
        recipientEmailPlaceholder: 'test&#64;example.com'
      },
      soraS3: {
        title: 'Sora Storage',
        description: 'Manage Sora media storage profiles with S3 and Google Drive support',
        newProfile: 'New Profile',
        reloadProfiles: 'Reload Profiles',
        empty: 'No storage profiles yet, create one first',
        createTitle: 'Create Storage Profile',
        editTitle: 'Edit Storage Profile',
        selectProvider: 'Select Storage Type',
        providerS3Desc: 'S3-compatible object storage',
        providerGDriveDesc: 'Google Drive cloud storage',
        profileID: 'Profile ID',
        profileName: 'Profile Name',
        setActive: 'Set as active after creation',
        saveProfile: 'Save Profile',
        activateProfile: 'Activate',
        profileCreated: 'Storage profile created',
        profileSaved: 'Storage profile saved',
        profileDeleted: 'Storage profile deleted',
        profileActivated: 'Active storage profile switched',
        profileIDRequired: 'Profile ID is required',
        profileNameRequired: 'Profile name is required',
        profileSelectRequired: 'Please select a profile first',
        endpointRequired: 'S3 endpoint is required when enabled',
        bucketRequired: 'Bucket is required when enabled',
        accessKeyRequired: 'Access Key ID is required when enabled',
        deleteConfirm: 'Delete storage profile {profileID}?',
        columns: {
          profile: 'Profile',
          profileId: 'Profile ID',
          name: 'Name',
          provider: 'Type',
          active: 'Active',
          endpoint: 'Endpoint',
          bucket: 'Bucket',
          storagePath: 'Storage Path',
          capacityUsage: 'Capacity / Used',
          capacityUnlimited: 'Unlimited',
          videoCount: 'Videos',
          videoCompleted: 'completed',
          videoInProgress: 'in progress',
          quota: 'Default Quota',
          updatedAt: 'Updated At',
          actions: 'Actions',
          rootFolder: 'Root folder',
          testInTable: 'Test',
          testingInTable: 'Testing...',
          testTimeout: 'Test timed out (15s)'
        },
        enabled: 'Enable Storage',
        enabledHint: 'When enabled, Sora generated media files will be automatically uploaded',
        endpoint: 'S3 Endpoint',
        region: 'Region',
        bucket: 'Bucket',
        prefix: 'Object Prefix',
        accessKeyId: 'Access Key ID',
        secretAccessKey: 'Secret Access Key',
        secretConfigured: '(Configured, leave blank to keep)',
        cdnUrl: 'CDN URL',
        cdnUrlHint: 'Optional. When configured, files are accessed via CDN URL',
        forcePathStyle: 'Force Path Style',
        defaultQuota: 'Default Storage Quota',
        defaultQuotaHint: 'Default quota when not specified at user or group level. 0 means unlimited',
        testConnection: 'Test Connection',
        testing: 'Testing...',
        testSuccess: 'Connection test successful',
        testFailed: 'Connection test failed',
        saved: 'Storage settings saved successfully',
        saveFailed: 'Failed to save storage settings',
        gdrive: {
          authType: 'Authentication Method',
          serviceAccount: 'Service Account',
          clientId: 'Client ID',
          clientSecret: 'Client Secret',
          clientSecretConfigured: '(Configured, leave blank to keep)',
          refreshToken: 'Refresh Token',
          refreshTokenConfigured: '(Configured, leave blank to keep)',
          serviceAccountJson: 'Service Account JSON',
          serviceAccountConfigured: '(Configured, leave blank to keep)',
          folderId: 'Folder ID (optional)',
          authorize: 'Authorize Google Drive',
          authorizeHint: 'Get Refresh Token via OAuth2',
          oauthFieldsRequired: 'Please fill in Client ID and Client Secret first',
          oauthSuccess: 'Google Drive authorization successful',
          oauthFailed: 'Google Drive authorization failed',
          closeWindow: 'This window will close automatically',
          processing: 'Processing authorization...',
          testStorage: 'Test Storage',
          testSuccess: 'Google Drive storage test passed (upload, access, delete all OK)',
          testFailed: 'Google Drive storage test failed'
        }
      }
      // openaiFastPolicy removed: the new upstream copy in
      // locales/en/admin/settings.ts (target models / other models action)
      // is what the current UI expects; the old fork copy was outdated.
    }
  },
  team: {
    consumer: {
      createTitle: 'Create Consumer',
      editTitle: 'Edit Consumer',
      name: 'Name',
      namePlaceholder: 'Enter consumer name',
      email: 'Email',
      emailPlaceholder: 'Enter consumer email',
      phone: 'Phone',
      phonePlaceholder: 'Enter consumer phone',
      title: 'Title',
      titlePlaceholder: 'Enter consumer title',
      type: 'Type',
      department: 'Department',
      selectDepartment: 'Select department',
      noDepartments: 'No departments available',
      status: 'Status',
      statusActive: 'Active',
      statusInactive: 'Inactive',
      types: {
        person: 'Person',
        application: 'Application',
        serviceAccount: 'Service Account'
      },
      errors: {
        nameRequired: 'Name is required',
        typeRequired: 'Type is required',
        departmentRequired: 'Department is required'
      }
    }
  },
  onboarding: {
    admin: {
      accountManage: {
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;"><b>Great! Group created successfully 🎉</b></p><p style="margin-bottom: 12px;">Now add upstream AI service accounts to enable actual service delivery.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>🔑 Account Purpose:</b><ul style="margin: 8px 0 0 16px;"><li>Connect to upstream AI services (DeepSeek、Kimi、Glm、Minimax, etc.)</li><li>One group can contain multiple accounts (load balancing)</li><li>Supports OAuth and Session Key methods</li></ul></div><p style="margin-top: 16px; color: #10b981; font-weight: 600;">👉 Click "Account Management" on the left sidebar</p></div>'
      }
    }
  },
  payment: {
    admin: {
      validityDays: 'Validity (days)',
      validityDaysRequired: 'Validity days must be greater than 0'
    }
  },
  legalDocument: {
    login: 'Login',
    loginTerms: 'Login Terms',
    loadError: 'Failed to load document',
    loadErrorDesc: 'Please refresh the page and try again.',
    notFound: 'Document Not Found',
    notFoundDesc: 'The requested legal document does not exist or has been removed by the administrator.',
    updatedAt: 'Updated: ',
    noContent: 'No content available',
    terms: 'Terms of Service',
    "usage-policy": 'Usage Policy',
    "supported-regions": 'Supported Countries and Regions',
    "service-specific-terms": 'Service-Specific Terms'
  },
  governance: {
    title: 'AI Governance & Compliance',
    description: 'Manage your AI governance and compliance settings.'
  },
  dataRights: {
    title: 'Data Subject Rights',
    description: 'Manage your data subject rights: export personal data, request data deletion, and manage consent records.',
    export: {
      title: 'Data Export',
      description: 'Under GDPR Article 20, you can export your personal data.',
      button: 'Export Data',
      processing: 'Processing...',
      success: 'Export request submitted, Export ID: {id}',
      successMessage: 'Data export request submitted',
      error: 'Failed to export data'
    },
    erasure: {
      title: 'Data Erasure',
      description: 'Under GDPR Article 17, you can request deletion of your personal data.',
      reasonLabel: 'Reason',
      reasonPlaceholder: 'Please explain why you want your data deleted...',
      confirmLabel: 'Confirmation Text',
      confirmPlaceholder: 'Type "DELETE MY DATA" to confirm',
      confirmHint: 'Type DELETE MY DATA to confirm deletion',
      submit: 'Submit Deletion Request',
      submitting: 'Submitting...',
      success: 'Erasure request submitted, Request ID: {id}',
      successMessage: 'Data erasure request submitted',
      error: 'Failed to submit erasure request',
      history: 'Erasure Request History',
      historyDescription: 'View the status of your submitted data erasure requests.',
      noRequests: 'No erasure requests',
      request: 'Erasure Request',
      requestType: 'Request Type',
      reason: 'Reason',
      status: {
        pending: 'Pending',
        approved: 'Approved',
        rejected: 'Rejected',
        completed: 'Completed'
      }
    },
    consent: {
      title: 'Consent Records',
      description: 'View and manage your data processing consent records.',
      empty: 'No consent records',
      version: 'Version',
      status: 'Status',
      granted: 'Granted',
      revoked: 'Revoked',
      grantedAt: 'Granted At',
      createdAt: 'Created At',
      loadError: 'Failed to load consent records',
      updateSuccess: 'Consent record updated',
      updateError: 'Failed to update consent record',
      types: {
        terms_of_service: {
          label: 'Terms of Service',
          description: 'I have read and agree to the Terms of Service (required)'
        },
        gdpr_data_processing: {
          label: 'GDPR Data Processing Agreement',
          description: 'I consent to the processing of my personal data in accordance with the GDPR Data Processing Agreement (required)'
        },
        detailed_logging: {
          label: 'Detailed Logging',
          description: 'Allow the system to record detailed request and response logs for auditing and troubleshooting'
        },
        cross_border_transfer: {
          label: 'Cross-border Data Transfer',
          description: 'Allow data transfer between different countries/regions to provide services'
        },
        marketing: {
          label: 'Marketing',
          description: 'Receive product updates, promotions, and other marketing communications'
        },
        model_training: {
          label: 'Model Training Data',
          description: 'Allow your data to be used for AI model improvement and optimization'
        }
      }
    }
  },
  compliance: {
    title: 'Account Compliance Config',
    description: 'Configure your Account-level AI governance and compliance policies, including industry templates, ZDR mode, compliance frameworks, and content moderation rules.',
    template: {
      title: 'Industry Template',
      description: 'Choose a compliance template suitable for your industry to quickly apply predefined compliance policies.',
      apply: 'Apply Template',
      current: 'Current Template',
      notApplied: 'Not Applied',
      industries: {
        ecommerce: {
          label: 'E-commerce',
          description: 'E-commerce industry compliance template: Recommendation engine user profiling notice, data retention 90 days.'
        },
        finance: {
          label: 'Financial Services',
          description: 'Financial industry compliance template: Credit scoring human oversight, audit trail, anti-fraud, data retention 365 days.'
        },
        healthcare: {
          label: 'Healthcare',
          description: 'Healthcare industry compliance template: Medical advice human oversight, HIPAA compliance, patient data protection, data retention 730 days.'
        },
        education: {
          label: 'Education',
          description: 'Education industry compliance template: Learning assessment human oversight, minor data protection, data retention 180 days.'
        }
      }
    },
    zdr: {
      title: 'ZDR Settings',
      description: 'Configure Zero Data Retention (ZDR) mode to control data retention policies.',
      mode: 'ZDR Mode',
      aggregate_only: 'Aggregate Only',
      audit: 'Audit',
      retention_days: 'Detail Log Retention Days'
    },
    frameworks: {
      title: 'Compliance Frameworks',
      description: 'Select applicable compliance frameworks to ensure your Account meets relevant regulatory requirements.',
      gdpr: 'GDPR',
      eu_ai_act: 'EU AI Act',
      hipaa: 'HIPAA',
      active: 'active'
    },
    moderation: {
      title: 'Content Moderation Policy',
      description: 'Manage content moderation rules, enable or disable specific moderation policies.',
      enabled: 'Enabled',
      enabledRules: 'rules enabled'
    },
    customRules: {
      title: 'Custom Moderation Rules',
      description: 'Create your own content moderation rules to supplement the system defaults.',
      create: 'New Rule',
      createTitle: 'Create Custom Rule',
      editTitle: 'Edit Custom Rule',
      edit: 'Edit',
      delete: 'Delete',
      update: 'Save',
      enabled: 'Enabled',
      disabled: 'Disabled',
      enableRule: 'Enable Rule',
      empty: 'No custom rules yet',
      ruleName: 'Rule Name',
      ruleNamePlaceholder: 'e.g. Internal Banned Words',
      ruleType: 'Rule Type',
      rulePattern: 'Match Pattern',
      patternPlaceholder: 'e.g. banned|sensitive (use | for multiple REGEX keywords)',
      action: 'Action',
      riskCategory: 'Risk Category',
      riskCategoryPlaceholder: 'e.g. Custom',
      createSuccess: 'Custom rule created successfully',
      updateSuccess: 'Custom rule updated successfully',
      deleteSuccess: 'Custom rule deleted successfully',
      deleteConfirm: 'Are you sure you want to delete this custom rule?'
    },
    status: {
      title: 'Config Status',
      description: 'Overview of your account\'s compliance configuration status.'
    },
    jurisdiction: {
      title: 'Jurisdiction Mapping',
      description: 'Automatically map applicable compliance regulations and requirements based on your business region, industry, and service type.',
      region: 'Company Region',
      industry: 'Industry',
      industryPlaceholder: 'e.g. Healthcare, Financial Services',
      serviceType: 'Service Type',
      map: 'Start Mapping',
      riskLevel: 'Risk Level',
      regulations: 'Applicable Regulations',
      measures: 'Required Measures',
      actions: 'Recommended Actions',
      fieldHelp: 'Field Help',
      fieldHelpRegion: 'The business region where you operate, used to identify applicable data protection regulations (e.g., GDPR, China PIPL)',
      fieldHelpIndustry: 'Your industry sector, which affects risk assessment levels and compliance requirements (e.g., healthcare, finance, education have special requirements)',
      fieldHelpServiceType: 'The type of AI service you provide, helping the system identify specific compliance obligations and assessment criteria. Options: AI Chatbot (AI chatbot service), AI Analysis (AI data analysis service), AI Recommendation (AI recommendation service)',
      applyRules: 'Auto-apply to compliance rules',
      save: 'Save Mapping',
      saved: 'Mapping saved',
      appliedRules: 'Applied rules',
      saveSuccess: 'Mapping saved successfully',
      applied: 'Applied',
      notApplied: 'Not Applied'
    },
    dpa: {
      title: 'DPA Compliance Statement',
      description: 'Generate DPA Compliance Statement to demonstrate GDPR Art.28 compliance elements. This is a compliance declaration, not a legally binding contract.',
      controllerName: 'Data Controller Name',
      controllerNamePlaceholder: 'Enter your company name',
      controllerContact: 'Data Controller Contact',
      controllerContactPlaceholder: 'Enter contact email or phone',
      generate: 'Generate DPA',
      success: 'DPA Compliance Statement generated successfully, file downloaded.'
    },
    credentials: {
      title: 'Compliance Credentials',
      description: 'View your compliance credentials, including certifications, audit reports, etc.',
      empty: 'No compliance credentials yet',
      validFrom: 'Valid From',
      validUntil: 'Valid Until',
      scope: 'Scope',
      issuerType: 'Issuer Type',
      credentialTypes: {
        GDPR_COMPLIANCE: 'GDPR Compliance Credential',
        EU_AI_ACT_ASSESSMENT: 'EU AI Act Assessment',
        ZERO_DATA_RETENTION: 'Zero Data Retention',
        DPA_COMPLIANCE: 'Data Processing Agreement (DPA)',
        SECURITY_CERTIFICATION: 'Security Certification'
      },
      issuerTypes: {
        SELF_ASSERTION: 'Self-Assertion',
        THIRD_PARTY: 'Third-Party'
      },
      metadata: {
        title: 'Details',
        compliance_basis: 'Compliance Basis',
        data_processing_record: 'Processing Record',
        dpo_contact: 'DPO Contact',
        data_retention: 'Data Retention',
        assessment_date: 'Assessment Date',
        risk_category: 'Risk Category',
        human_in_the_loop: 'Human in the Loop',
        model_training: 'Model Training',
        prompt_storage: 'Prompt Storage',
        policy_version: 'Policy Version',
        request_content: 'Request Content',
        technical_logs: 'Technical Logs',
        security_logs: 'Security Logs',
        backup_policy: 'Backup Policy',
        dpa_version: 'DPA Version',
        scc_compliant: 'SCC Compliant',
        subprocessor_approval_required: 'Subprocessor Approval Required',
        data_subject_rights_supported: 'Supported Data Subject Rights',
        encryption_at_rest: 'Encryption at Rest',
        transport_security: 'Transport Security',
        access_control: 'Access Control',
        audit_logging: 'Audit Logging',
        security_audits: 'Security Audits'
      }
    },
    audit: {
      title: 'Audit Logs',
      description: 'View your compliance operation audit logs, recording all compliance-related activities.',
      empty: 'No audit logs yet',
      operator: 'Operator',
      page: 'Page',
      prev: 'Previous',
      next: 'Next'
    },
    risk: {
      title: 'Risk Analysis',
      description: 'View the risk tag catalog supported by the system, understand model risk and data risk classifications.',
      modelTags: 'Model Tags',
      riskTags: 'Risk Tags',
      tags: {
        MODEL_FRONTIER: {
          label: 'Frontier Model',
          description: 'Using frontier AI models'
        },
        MODEL_OPEN_SOURCE: {
          label: 'Open Source Model',
          description: 'Using open source models'
        },
        MODEL_EXTERNAL_PROVIDER: {
          label: 'External Provider',
          description: 'Using models from external providers'
        },
        MODEL_DATA_RETENTION_UNKNOWN: {
          label: 'Unknown Data Retention',
          description: 'Model provider data retention policy is unknown'
        },
        PII_DETECTED: {
          label: 'PII Detected',
          description: 'Personal identifiable information detected'
        },
        HIGH_RISK_USE_CASE: {
          label: 'High Risk Use Case',
          description: 'High risk application scenario'
        },
        CROSS_BORDER_TRANSFER: {
          label: 'Cross-Border Transfer',
          description: 'Cross-border data transfer'
        },
        SANCTIONED_REGION: {
          label: 'Sanctioned Region',
          description: 'Access from sanctioned region'
        },
        CONTENT_POLICY_VIOLATION: {
          label: 'Content Policy Violation',
          description: 'Content policy violation detected'
        },
        OUTPUT_CONTROL_LIMITED: {
          label: 'Output Control Limited',
          description: 'Output control is limited'
        },
        NO_TRAINING_GUARANTEE: {
          label: 'No Training Guarantee',
          description: 'No training data guarantee'
        },
        RATE_LIMIT_EXCEEDED: {
          label: 'Rate Limit Exceeded',
          description: 'Rate limit exceeded'
        },
        ANOMALOUS_BEHAVIOR: {
          label: 'Anomalous Behavior',
          description: 'Anomalous behavior detected'
        }
      }
    },
    euAiAct: {
      title: 'EU AI Act Assessment',
      description: 'View your AI system compliance assessment report for EU AI Act.',
      export: 'Export Assessment',
      exportSuccess: 'Assessment report exported successfully.'
    },
    ropa: {
      title: 'GDPR Processing Records',
      description: 'View GDPR Art 30 data processing activity records (ROPA), meeting compliance audit requirements.'
    },
    login: 'Login',
    hero: {
      badge: 'Enterprise-Grade AI Compliance Solution',
      title: 'Compliance-First AI',
      subtitle: 'ThreeRouter provides a comprehensive AI governance and compliance framework, helping you confidently navigate EU AI Act, GDPR, and global regulatory requirements.',
      ctaPrimary: 'Try Now',
      ctaSecondary: 'Learn More'
    },
    features: {
      euai: {
        title: 'EU AI Act Compliance Assessment',
        description: 'Professional AI system role identification and legal mapping, Annex III-based high-risk scenario assessment.',
        highlight1: 'AI system role identification and legal mapping',
        highlight2: 'Annex III-based high-risk scenario assessment',
        highlight3: 'GPAI downstream integrator compliance framework',
        highlight4: 'Article 50 transparency obligations fully disclosed'
      },
      gdpr: {
        title: 'GDPR Art.30 ROPA',
        description: 'Complete Records of Processing Activities (Processor Activities), compliant legal basis declarations.',
        highlight1: 'Complete records of processing activities',
        highlight2: 'Compliant legal basis declarations',
        highlight3: 'EU SCC cross-border transfer mechanisms',
        highlight4: 'Data subject rights support'
      },
      zdr: {
        title: 'Zero Data Retention Architecture',
        description: 'Default no request content retention (ZDR), flexible data retention policy configuration.',
        highlight1: 'Default no request content retention',
        highlight2: 'Flexible data retention policy configuration',
        highlight3: 'Data minimization principles enforced',
        highlight4: 'Aggregate/Audit/Detail three-level modes'
      },
      hipaa: {
        title: 'HIPAA Healthcare Compliance',
        description: 'Healthcare industry-specific compliance framework, supporting medical advice human oversight and patient data protection.',
        highlight1: 'Medical advice human oversight mechanism',
        highlight2: 'Strict patient data protection',
        highlight3: '730-day compliant data retention period',
        highlight4: 'Complete audit trail capabilities'
      },
      credentials: {
        title: 'One-Stop Compliance Credentials',
        description: 'Five compliance certificates auto-generated, one-click export compliance reports.',
        highlight1: 'GDPR Compliance Certificate',
        highlight2: 'EU AI Act Assessment Report',
        highlight3: 'Zero Data Retention Statement',
        highlight4: 'Data Processing Agreement (DPA)'
      },
      templates: {
        title: 'Industry Compliance Templates',
        description: 'Four industry templates for Healthcare, Finance, Education, and E-commerce, pre-configured rules ready to use.',
        highlight1: 'Healthcare industry compliance template',
        highlight2: 'Financial services industry compliance template',
        highlight3: 'Education industry compliance template',
        highlight4: 'E-commerce industry compliance template'
      },
      risk: {
        title: 'Risk Analysis & Monitoring',
        description: 'Real-time risk tag monitoring, anomaly detection, ensuring AI application security and compliance.',
        highlight1: 'Real-time risk tag monitoring',
        highlight2: 'Anomaly detection',
        highlight3: 'Compliance policy violation alerts',
        highlight4: 'Audit log traceability'
      }
    },
    certificates: {
      title: 'Compliance Credentials',
      description: 'One-stop compliance credential management, auto-generated, one-click export',
      gdpr: 'GDPR Compliance Certificate',
      euai: 'EU AI Act Assessment Report',
      zdr: 'Zero Data Retention Statement',
      dpa: 'Data Processing Agreement',
      hipaa: 'HIPAA Healthcare Compliance Statement',
      security: 'Security Certification'
    },
    templates: {
      title: 'Industry Compliance Templates',
      description: 'Pre-configured compliance policies, ready to use',
      apply: 'Apply Template',
      healthcare: 'Healthcare',
      healthcareDesc: 'Medical advice human oversight, HIPAA compliance, patient data protection, 730 days retention',
      finance: 'Financial Services',
      financeDesc: 'Credit scoring human oversight, audit trail, anti-fraud, 365 days retention',
      education: 'Education',
      educationDesc: 'Learning assessment human oversight, minor data protection, 180 days retention',
      ecommerce: 'E-commerce',
      ecommerceDesc: 'Recommendation engine user profiling notice, 90 days retention'
    },
    cta: {
      title: 'Start Your AI Compliance Journey',
      description: 'Experience ThreeRouter enterprise-grade AI governance and compliance solution now, making AI compliance simple.',
      primary: 'Try Now',
      secondary: 'Login to Console'
    },
    samples: {
      title: 'Report Samples',
      description: 'View our compliance report samples to understand how ThreeRouter helps enterprises meet regulatory requirements',
      euAiAct: {
        title: 'EU AI Act Assessment Report Sample',
        description: 'View complete AI system role identification and high-risk assessment'
      },
      gdprRopa: {
        title: 'GDPR ROPA Report Sample',
        description: 'View complete records of processing activities and legal basis declarations'
      }
    },
    footer: '© 2026 ThreeRouter. All rights reserved.'
  },
  whitepaper: {
    type: 'Whitepaper',
    title: 'ThreeRouter AI Governance & Compliance Whitepaper',
    subtitle: 'Learn how ThreeRouter helps enterprises navigate EU AI Act, GDPR, and global AI regulatory requirements, building secure and compliant AI application architectures.',
    downloadBtn: 'Download PDF Whitepaper',
    download: 'Whitepaper download feature coming soon. Stay tuned for updates.',
    back: 'Back to Compliance Page',
    toc: 'Table of Contents',
    footer: '© 2026 ThreeRouter. All rights reserved.',
    chapters: {
      intro: {
        title: 'Chapter 1: The Challenges of AI Regulation Era',
        description: 'With the introduction of EU AI Act, GDPR, and other regulations, enterprises face unprecedented AI compliance challenges.',
        section1: '1.1 Global AI Regulatory Trends',
        content1: 'The European Union AI Act (EU AI Act) is the world\'s first comprehensive AI regulation, classifying AI systems into four risk categories and requiring rigorous compliance assessments for high-risk AI systems. At the same time, GDPR requirements for data processing activities are becoming increasingly strict, requiring enterprises to maintain comprehensive Records of Processing Activities (ROPA) to meet audit requirements.',
        section2: '1.2 Compliance Challenges for Enterprises',
        content2: 'Enterprises face multiple compliance challenges in AI applications: How to determine the legal role of AI systems? How to assess AI system risk levels? How to ensure cross-border data transfer compliance? How to manage compliance responsibilities of third-party AI model providers?',
        section3: '1.3 ThreeRouter\'s Solution',
        content3: 'ThreeRouter provides a complete AI governance and compliance framework to help enterprises address the above challenges. Through built-in compliance capabilities, enterprises can quickly complete EU AI Act assessments, GDPR ROPA preparation, Data Processing Agreement generation, and other tasks.'
      },
      euai: {
        title: 'Chapter 2: EU AI Act Compliance Framework',
        description: 'Deep dive into ThreeRouter\'s EU AI Act compliance assessment system.',
        section1: '2.1 AI System Role Identification',
        content1: 'ThreeRouter helps enterprises clarify the legal role of AI systems under the EU AI Act. Through professional role mapping analysis, enterprises can determine whether they are AI system providers, deployers, or infrastructure service providers, avoiding compliance risks caused by legal role confusion.',
        section2: '2.2 High-Risk Scenario Assessment',
        content2: 'Based on the EU AI Act Annex III high-risk scenario list, ThreeRouter helps enterprises assess whether AI systems involve high-risk application scenarios. The assessment considers multiple dimensions including AI system purpose, data processing methods, and decision impacts.',
        section3: '2.3 GPAI Downstream Integrator Compliance',
        content3: 'For enterprises using third-party general-purpose AI models, ThreeRouter provides a downstream integrator compliance framework, clarifying the responsibility boundary between enterprises and model providers, ensuring compliance with EU AI Act special requirements for GPAI.'
      },
      gdpr: {
        title: 'Chapter 3: GDPR Compliance Practices',
        description: 'Complete GDPR compliance solutions, from processing activity records to data subject rights.',
        section1: '3.1 Records of Processing Activities (ROPA)',
        content1: 'ThreeRouter helps enterprises prepare processing activity records meeting GDPR Art.30 requirements. Records include processing activity descriptions, legal bases, data categories, recipients, cross-border transfer mechanisms, retention periods, and other key information.',
        section2: '3.2 Legal Basis Management',
        content2: 'Ensuring all data processing activities have legal bases is the core of GDPR compliance. ThreeRouter supports management of multiple legal bases, including contract performance, legitimate interest, legal obligation, etc., helping enterprises assess the applicability of legal bases.',
        section3: '3.3 Data Subject Rights Support',
        content3: 'ThreeRouter provides data subject rights support mechanisms, helping enterprises respond to requests for access, erasure, portability, restriction, and objection, ensuring compliance with GDPR requirements for protecting data subject rights.'
      },
      architecture: {
        title: 'Chapter 4: Zero Data Retention Architecture',
        description: 'Learn how ThreeRouter\'s unique zero data retention architecture protects data security.',
        section1: '4.1 ZDR Mode Principles',
        content1: 'Zero Data Retention (ZDR) is ThreeRouter\'s core security architecture. By default, API request and response content is not retained, only aggregated usage metrics are recorded, maximizing user data privacy protection.',
        section2: '4.2 Flexible Retention Policies',
        content2: 'ThreeRouter supports three data retention modes: Aggregate Only, Audit, and Detail. Enterprises can flexibly configure data retention policies based on their compliance needs.',
        section3: '4.3 Data Minimization Principle',
        content3: 'Data minimization is one of GDPR\'s fundamental principles. ThreeRouter implements the data minimization principle in its design, only collecting and processing data necessary to achieve service purposes, avoiding excessive collection of personal information.'
      },
      implementation: {
        title: 'Chapter 5: Compliance Implementation Guide',
        description: 'How to implement AI compliance in actual business operations.',
        section1: '5.1 Compliance Assessment Process',
        content1: 'ThreeRouter provides a complete compliance assessment process: First conduct AI system role and risk assessment, then select applicable industry compliance templates, configure data retention policies, and finally generate compliance credentials and reports.',
        section2: '5.2 Industry Compliance Template Application',
        content2: 'For Healthcare, Financial Services, Education, E-commerce, and other industries, ThreeRouter provides pre-configured compliance templates containing industry-specific compliance requirements and best practices, helping enterprises quickly meet industry compliance standards.',
        section3: '5.3 Continuous Compliance Monitoring',
        content3: 'Compliance is not a one-time task but a continuous process. ThreeRouter provides real-time risk monitoring and compliance status tracking, helping enterprises promptly identify compliance risks and ensure AI applications always meet regulatory requirements.'
      }
    }
  },
  enterprise: {
    nav: {
      enterprise: 'Enterprise',
      authority: 'Authority Insights',
      compare: 'Compare',
      cases: 'Cases',
      cta: 'Create Team Account',
      login: 'Login',
      backHome: 'Back to Home',
    },
    hero: {
      badge: 'Enterprise AI API Management',
      title: 'Enterprise-Ready Access to the World\'s Top AI Models',
      subtitle: 'Unified team management · Secure · Compliant · Trusted',
      roleLabel: 'Choose Your Role, Get Your Solution',
      roleDecisionTitle: 'I am a Decision Maker',
      roleDecisionDesc: 'CEO / CTO / Tech Lead / Procurement Manager',
      roleDecisionFocus: 'Focus on security, compliance, and cost control',
      roleDecisionTag: 'Enterprise Procurement Plan',
      roleEmployeeTitle: 'I am an Employee',
      roleEmployeeDesc: 'Developer / Product Manager / Designer',
      roleEmployeeFocus: 'Focus on using top AI models and team collaboration',
      roleEmployeeTag: 'Recommend to Company',
      decisionTitle: 'For Decision Makers',
      decisionDesc: 'You need a secure, compliant, and cost-controllable API solution. ThreeRouter provides enterprise-grade security, team quota management, and transparent usage control.',
      employeeTitle: 'For Team Members',
      employeeDesc: 'Recommend ThreeRouter to your company. One application, whole team benefits. Unified API gateway with one-line model switching.',
      ctaPrimary: 'Create Team Account →',
      ctaSecondary: 'Compare with Relay Stations',
      ctaEmployee: 'Recommend to Company →',
      statCustomers: 'Enterprise Customers',
      statRequests: 'Total API Calls',
      statUptime: 'Uptime SLA',
      statLeaks: 'Data Leak Incidents',
      statCustomersValue: '5,000+',
      statRequestsValue: '5B+',
      statUptimeValue: '99.9%',
      statLeaksValue: '0'
    },
    authority: {
      label: 'Industry Insights',
      title: 'In the AI Era, Enterprises Need a Unified Token Platform',
      subtitle: 'In the future, companies buy time with Tokens. Equipping employees with Token allowances will become a standard corporate benefit, just like computers and desks.',
      quote1: '"In the past, companies bought time with people; in the future, they buy time with Tokens."',
      quote1Desc: 'The value-creation formula of the AI era is being rewritten. The most expensive asset is not the coder, but the person who masters AI tools.',
      quote2: '"Token is becoming the currency of AI-native enterprises."',
      quote2Desc: 'Providing employees with a unified AI Token account is safer, cheaper, and more controllable than letting them purchase individually.',
      quote3: '"AI-native companies only need 2 people: CEO and CTO."',
      quote3Desc: 'Every employee will have an AI assistant, and Token is their productivity fuel. Bulk Token procurement by enterprises is inevitable.',
      cta: 'Explore Enterprise Plan →'
    },
    pain: {
      label: 'Market Reality',
      title: 'Three Things Relay Stations Won\'t Tell You',
      subtitle: 'The Token API market is flooded with low-price relay stations that hide risks of data theft, model substitution, and sudden shutdowns.',
      item1Title: 'Data Exposed, Relay Stations Copy Everything',
      item1Desc: 'Every conversation and business data passes through relay servers. They can store, analyze, and resell it all. Dozens of relay data leaks have occurred this year.',
      item2Title: 'Model Switching, You Pay for Pro and Get ',
      item2Desc: 'It is an open secret in the relay industry: the frontend shows Opus while the backend swaps in Haiku or even open-source models. Up to 68% of requests are downgraded in practice.',
      item3Title: 'Sudden Shutdowns, No Invoice at All',
      item3Desc: 'Many relay stations are run by individuals without business registration, compliant invoices, or SLAs. Over 30 well-known relay stations suddenly shut down this year.'
    },
    solution: {
      label: 'ThreeRouter Solution',
      title: 'Peace of Mind for CEOs, Less Hassle for CTOs',
      subtitle: 'Reach every role in the enterprise decision chain with one solution that meets the whole team\'s needs.',
      card1Badge: 'Zero Risk Security & Compliance',
      card1Title: 'Token Is Productivity, Spend Is ROI',
      card1Desc: 'API access is not a cost, but an investment. Give your team top models and boost R&D efficiency 10x.',
      card1Points: [
        'Visualized usage and transparent costs',
        'Corporate transfer and compliant invoices',
        'Bulk procurement saves 30%+ vs individual purchase'
      ],
      card1Btn: 'Book a Demo',
      card2Badge: 'Save 30% with Transparent Costs',
      card2Title: 'Secure, Compliant, Zero Data Risk',
      card2Desc: 'Choose a compliant, transparent, and auditable API path so security risks no longer keep you up at night.',
      card2Points: [
        'No data storage, relay, or cross-border transfer',
        'Transparent models, no downgrade promised',
        'Enterprise SLA guarantee (99.9%)'
      ],
      card2Btn: 'View Documentation',
      card2BtnLink: '/help-en.html',
      card3Badge: 'Unified API for Efficiency',
      card3Title: 'One Application, Whole Team Uses Top Models',
      card3Desc: 'Recommend the company to adopt ThreeRouter, unify the API gateway, and switch models with one line of code.',
      card3Points: [
        'Unified API compatible with OpenAI format',
        'Company pays centrally, no individual top-up',
        'DeepSeek, Kimi, Glm, Minimax all in one place'
      ],
      card3Btn: 'Recommend to Company'
    },
    team: {
      label: 'Team Management',
      title: 'Intelligent Team Management Puts Every API Call in Control',
      subtitle: 'Departments, consumers, keys, usage, and costs — all managed in one place with full visibility.',
      feature1Title: 'Hierarchical Department Management',
      feature1Desc: 'Organize your team with a clear department tree. Create nested departments, assign department codes, and visualize reporting lines at a glance.',
      feature1Points: [
        'Tree-based department visualization',
        'Custom department codes for cost allocation',
        'Department-level usage and cost tracking'
      ],
      feature2Title: 'Consumer Lifecycle Management',
      feature2Desc: 'Manage every API consumer from creation to retirement. Assign consumers to departments, set types and roles, and track activity in real time.',
      feature2Points: [
        'streamlined form for quick onboarding',
        'Type-first selection for clear categorization',
        'Full profile: name, email, phone, position, department'
      ],
      feature3Title: 'Smart API Key Management',
      feature3Desc: 'Link API keys to specific departments and consumers for granular access control. Selecting a department automatically filters available consumers.',
      feature3Points: [
        'Department-aware consumer filtering',
        'Secure key generation and rotation',
        'Per-key usage and cost attribution'
      ],
      feature4Title: 'Comprehensive Team Analytics',
      feature4Desc: 'Make data-driven decisions with rich, multi-dimensional analytics. Switch between daily and hourly granularity, and choose any date range.',
      feature4Points: [
        'Usage trends: daily or hourly granularity',
        'Platform distribution: see resource consumption by AI platform',
        'Consumer rankings: identify top consumers',
        'Cost insights: requests, tokens, and costs at a glance'
      ],
      feature5Title: 'Role-Based Team Member Management',
      feature5Desc: 'Control who can do what with granular role assignments. Add admins, invite members, and manage pending invitations.',
      feature5Points: [
        'Admin and member role system',
        'Invitation workflow with pending tracking',
        'Active member statistics at a glance'
      ],
      feature6Title: 'Real-Time Cost Optimization',
      feature6Desc: 'Every request is tracked, every token is counted, every cost is attributed. With a 1-minute aggregation engine, your dashboard reflects real-time usage.',
      feature6Points: [
        '1-minute aggregation for near real-time insights',
        'Per-consumer, per-department, per-team cost breakdowns',
        'Historical trend analysis for budget planning'
      ]
    },
    tokenManagement: {
      label: 'Fine-Grained Control',
      title: 'Enterprise Token Usage Fine-Grained Management',
      subtitle: 'Every token is tracked, every call is logged, every cost is attributed. Full-chain visibility from department to individual, from model to request.',
      metrics: [
        {
          value: '1min',
          label: 'Data Aggregation',
          desc: 'Near real-time insights'
        },
        {
          value: '6-axis',
          label: 'Cost Breakdown',
          desc: 'Team/Dept/Consumer/Model/Key/Request'
        },
        {
          value: '100%',
          label: 'Cost Attribution',
          desc: 'Every expense traceable'
        },
        {
          value: '0-gap',
          label: 'Data Completeness',
          desc: 'No call missed'
        }
      ],
      features: [
        {
          icon: '🎯',
          iconBg: 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300',
          title: 'Multi-Dimensional Usage Tracking',
          desc: 'Track token consumption across model, consumer, department, key, and time dimensions.',
          points: [
            'Model-level token consumption ranking',
            'Consumer-level granular usage attribution',
            'Department-level cost aggregation view',
            'Time-series trend analysis (daily/hourly)'
          ]
        },
        {
          icon: '💰',
          iconBg: 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300',
          title: 'Intelligent Cost Control',
          desc: 'Budget setting, quota allocation, and overspend alerts keep every AI investment accountable.',
          points: [
            'Per-department/consumer token quota limits',
            'Automatic budget overrun alerts',
            'Cost trend forecasting and budget planning',
            'Real-time balance and burn rate monitoring'
          ]
        },
        {
          icon: '🔑',
          iconBg: 'bg-purple-100 text-purple-600 dark:bg-purple-900/30 dark:text-purple-300',
          title: 'Key-Level Granular Control',
          desc: 'Each API key has independent billing, quotas, and auditing for true fine-grained management.',
          points: [
            'Per-key usage and cost accounting',
            'Key-level quota and rate limits',
            'Key lifecycle management (create/rotate/revoke)',
            'Key-to-department/consumer traceability'
          ]
        },
        {
          icon: '📊',
          iconBg: 'bg-cyan-100 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-300',
          title: 'Real-Time Dashboards',
          desc: '1-minute data aggregation engine provides near real-time visibility into team AI usage.',
          points: [
            'Near real-time usage data refresh',
            'Multi-dimensional cross-analysis panels',
            'Exportable customized reports',
            'API call trend visualization charts'
          ]
        },
        {
          icon: '🛡️',
          iconBg: 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300',
          title: 'Compliance Audit Trail',
          desc: 'Complete logging of every API call to meet internal audit and industry compliance requirements.',
          points: [
            'Full call log traceability',
            'Data processing activity records (GDPR compatible)',
            'Automated audit report generation',
            'End-to-end operation behavior tracking'
          ]
        },
        {
          icon: '⚡',
          iconBg: 'bg-rose-100 text-rose-600 dark:bg-rose-900/30 dark:text-rose-300',
          title: 'Intelligent Optimization Insights',
          desc: 'Automatically identifies optimization opportunities based on historical usage data.',
          points: [
            'High-frequency low-efficiency call detection',
            'Model selection optimization suggestions',
            'Idle resource reclamation reminders',
            'Real-time cost saving opportunity alerts'
          ]
        }
      ],
      processTitle: 'Four-Step Token Management',
      processSteps: [
        {
          title: 'Set Budgets & Quotas',
          desc: 'Allocate token budgets and usage quotas by department and consumer'
        },
        {
          title: 'Real-Time Monitoring',
          desc: '1-minute aggregation with full-dimension usage and cost monitoring'
        },
        {
          title: 'Alerts & Optimization',
          desc: 'Overrun alerts, anomaly detection, automated optimization suggestions'
        },
        {
          title: 'Audit & Review',
          desc: 'Generate audit reports, review AI ROI'
        }
      ]
    },
    teamWorkflow: {
      label: 'Quick Start',
      title: 'Create a Team, Manage Together',
      subtitle: 'Three steps to create a team and manage member permissions and usage quotas.',
      step1Title: 'Create Team Space',
      step1Desc: 'Administrators quickly create a team space, set up member permissions and usage quotas. Supports role-based granular control for security and compliance.',
      step1Points: [
        'One-click team space creation',
        'Granular member permission management',
        'Flexible usage quota configuration'
      ],
      step2Title: 'Invite to Join Team',
      step2Desc: 'Administrators generate an invitation link with one click, team members join instantly. No complicated registration, unified team billing and auditing.',
      step2Points: [
        'One-click invitation link generation',
        'Team members join and use instantly',
        'Unified billing and auditing'
      ],
      step3Title: 'Member & Quota Control',
      step3Desc: 'Add or remove team members, assign access permissions by role. Monitor usage in real time, set differentiated quotas, and allocate resources wisely.',
      step3Points: [
        'Flexible member management',
        'Real-time usage monitoring',
        'Differentiated quota allocation'
      ]
    },
    compare: {
      label: 'Transparent Comparison',
      title: 'ThreeRouter vs Ordinary Relay Stations',
      subtitle: 'See the difference at a glance. Choosing a Token API service is not only about price.',
      dimension: 'Dimension',
      us: 'ThreeRouter',
      them: 'Ordinary Relay Station',
      rows: {
        entity: {
          label: 'Operator',
          us: 'Official company operation',
          them: 'Often unregistered individual operations'
        },
        security: {
          label: 'Data Security',
          us: 'No conversation storage, end-to-end encryption',
          them: 'Can record and resell data'
        },
        transparency: {
          label: 'Model Transparency',
          us: 'Transparent and traceable',
          them: 'May downgrade models, unverifiable'
        },
        sla: {
          label: 'SLA Guarantee',
          us: '99.9% uptime with SLA',
          them: 'No SLA, may shut down anytime'
        },
        invoice: {
          label: 'Invoice',
          us: 'Compliant invoices, corporate transfer',
          them: 'Usually no invoice capability'
        },
        support: {
          label: 'Support',
          us: 'Professional team, dedicated contact',
          them: 'Essentially zero'
        },
        price: {
          label: 'Price',
          us: 'Market-reasonable pricing',
          them: 'Extremely low even 90% off'
        }
      }
    },
    employeeApply: {
      label: 'Bottom-Up',
      title: 'Apply to Your Boss in One Click, Let the Company Pay',
      subtitle: 'Spend company money, use the best AI models. Fill in the info and generate an application message for your boss.',
      cardTitle: 'Generate Application Message in One Step',
      generateBtn: 'Generate Application Message →',
      benefitsTitle: 'Why Should Your Boss Approve?',
      benefits: [
        {
          title: 'Why Should Your Boss Approve?',
          desc: 'ThreeRouter is an official product, secure, compliant, and reliable. The spend is an investment in team efficiency, not personal consumption.'
        },
        {
          title: 'How Much Can It Improve?',
          desc: 'After adopting AI APIs, code generation, debugging, and documentation writing can become 5-10x more efficient. Interns can deliver high-quality output.'
        },
        {
          title: 'Fully Compliant, Finance Worry-Free',
          desc: 'Supports corporate transfer and compliant invoices. No hidden corners, fully OK for finance audits.'
        },
        {
          title: 'One Person Applies, Whole Team Benefits',
          desc: 'Once the company enables it, the entire tech team can use the best AI models. One person drives, whole team upgrades.'
        }
      ],
      applicationLabel: 'Application Text',
      copyBtn: 'Copy',
      generatedLabel: 'Application Generated',
      copySuccess: 'Copied!',
      applicationTemplate: {
        greeting: 'Dear Boss,',
        intro: 'I recommend that our company open a ThreeRouter enterprise account for the tech team to centrally manage AI API calls. Here is why:',
        reason1: '1. Secure & Reliable: ThreeRouter provides enterprise-grade security and compliance. No data storage or relay, supports corporate invoicing.',
        reason2: '2. Cost Control: Centralized procurement is cheaper than individual purchases. Usage visualization for clear budgeting.',
        reason3: '3. Efficiency Boost: Unified API gateway compatible with OpenAI format. Switch between DeepSeek、Kimi、Glm、Minimax with one line of code.',
        reason4: '4. Team Management: One-stop management of departments, consumers, keys, usage, and costs for easy expansion.',
        closing: 'Thank you for your approval!'
      }
    },
    cases: {
      label: 'Success Cases',
      title: 'Forward-Looking Enterprises Are Already Using It',
      subtitle: 'More and more tech teams are choosing ThreeRouter.',
      case1Company: 'China Team of a Multinational Corporation',
      case1Title: 'Unified API Gateway, 50-Person Team Efficiency Up 5x',
      case1Desc: 'After adopting ThreeRouter, the entire tech team uses one unified API gateway instead of connecting to different models individually. O&M costs dropped 70%, model costs dropped 40%.',
      case2Company: 'Wuhan ## Technology Team',
      case2Title: 'Migrated from Personal Relay Station to ThreeRouter, Data Security Finally at Ease',
      case2Desc: 'Some team members previously used personal relay services, and the CTO worried about data security. After adopting ThreeRouter, all API calls are under enterprise-grade security.',
      case3Company: '## Cloud Tech Department',
      case1Initial: 'VC',
      case2Initial: 'WK',
      case3Initial: 'SC',
      case3Title: 'Stable and Reliable, Supporting Millions of Daily API Calls',
      case3Desc: 'Daily API calls exceed 1.2 million, and ThreeRouter maintains 99.95% availability. Professional team provides 7×24 response.'
    },
    cta: {
      label: 'Get Started',
      title: 'Get Your Team on Secure and Reliable AI APIs Today',
      subtitle: 'Choose the way that suits you best',
      primary: 'Create Team Account →',
      secondary: 'I am an employee, recommend to company →'
    },
    faq: {
      eyebrow: 'FAQ',
      title: 'Enterprise Service FAQ',
      subtitle: 'Common questions about enterprise onboarding, security & compliance, Token management, and team collaboration',
      q1: 'What is the enterprise onboarding process? How long does it take?',
      a1: 'Standard enterprise onboarding can be completed within 1 business day. The process includes: creating an enterprise account → configuring team structure and department quotas → distributing API Keys → making API calls. We provide full technical support throughout the migration.',
      q2: 'How do you ensure enterprise data security and compliance?',
      a2: 'We use end-to-end encrypted transport (TLS 1.3), do not store user request data or response content, and do not use data for model training. The platform supports comprehensive audit logs, meeting GDPR, EU AI Act, and other compliance requirements. All upstream resources come from official cloud providers like AWS, GCP, and Azure with legal and auditable procurement chains.',
      q3: 'What dimensions does Token refined management support?',
      a3: 'Token quota management is supported across four dimensions: department, team, project, and API Key. Each dimension can independently set usage limits, alert thresholds, and cost attribution. Administrators can view real-time Token consumption details by dimension and export reports by time period.',
      q4: 'Do you provide an SLA (Service Level Agreement)?',
      a4: 'Yes. Enterprise customers receive a 99.9% availability SLA. The platform uses multi-provider routing and failover mechanisms to ensure high availability. If the SLA commitment is not met, credit compensation will be provided as specified in the agreement.',
      q5: 'Do you support private deployment or dedicated instances?',
      a5: 'Yes. Enterprises can choose dedicated cloud instances (Dedicated Cloud) or full on-premise deployment. Dedicated instances provide isolated resource pools and network isolation. On-premise deployment supports deployment on enterprise-owned servers or private cloud environments.',
      q6: 'How do I integrate with existing systems and tech stacks?',
      a6: 'Fully compatible with the OpenAI API format — just replace the base URL and API Key. Supports all major SDKs (Python, Node.js, Go, Java), Agent frameworks (LangChain, AutoGen, CrewAI), and workflow tools. Migration requires only a one-line code change.',
      q7: 'What roles and granularity does team permission management support?',
      a7: 'Supports four built-in roles: Administrator, Department Lead, Developer, and Read-Only User, plus custom roles. Permission granularity covers model access, API Key management, usage viewing, quota settings, and billing management. Supports permission isolation by department and project.',
      q8: 'Do you provide dedicated enterprise technical support?',
      a8: 'Yes. Enterprise customers have access to a dedicated technical support channel, including 7x24 ticket response, enterprise chat group support, regular architecture reviews, and usage optimization recommendations. Large enterprises can be assigned a dedicated Customer Success Manager (CSM) for one-on-one service.'
    },
    footer: {
      brandDesc: 'ThreeRouter is a unified multi-model API management platform that helps teams quickly access the world\'s top AI models with secure, compliant, and stable enterprise-grade service.',
      copyright: '© 2026 ThreeRouter. All rights reserved.',
      slogan: 'Secure · Compliant · Trusted'
    }
  }
}
