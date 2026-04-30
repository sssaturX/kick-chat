export type Lang = 'ru' | 'en'

export const translations = {
  ru: {
    nav: {
      home: 'Главная',
      what: 'Продукт',
      features: 'Возможности',
      screenshots: 'Скриншоты',
      faq: 'FAQ',
      getSaturx: 'Получить SATURX',
    },
    cta: 'Получить доступ',
    hero: {
      eyebrow: 'Desktop tool for Kick',
      headline: 'SATURX объединяет Kick-чат, дашборд и контроль активности в одном окне.',
      subheadline:
        'Десктопный инструмент для стримеров и модераторов Kick: чат, аккаунты, заготовки сообщений, авто-отправка и управление viewer boost в чистом интерфейсе.',
      desc: 'Меньше переключений между вкладками. Больше контроля. Всё в SATURX.',
      button: 'Получить SATURX',
      learnMore: 'Смотреть возможности',
      proof: ['Быстро и легко', 'Безопасно по дизайну', 'Создан для Kick'],
      visualTitle: 'Превью дашборда',
      visualSubtitle: 'Чат, стрим-превью, аккаунты и инструменты активности в одном рабочем пространстве.',
      visualBadges: ['Чат', 'Аккаунты', 'Заготовки', 'Viewer-панель'],
    },
    strip: {
      title:
        'SATURX помогает держать рабочий процесс Kick в одном месте: чат, стрим, аккаунты, заготовки и инструменты активности без лишнего хаоса.',
    },
    cards: {
      tag: 'Продукт',
      mainTitle: 'Один дашборд вместо набора вкладок',
      mainDesc:
        'Работайте с Kick-чатом, аккаунтами, сообщениями и viewer boost controls в едином интерфейсе. SATURX создан для быстрых действий во время стрима.',
      multiTitle: 'Несколько аккаунтов',
      multiDesc: 'Переключайте аккаунты и отправляйте подготовленные сообщения из одного рабочего экрана.',
      licenseTitle: 'Доступ по лицензии',
      licenseDesc: 'Активация по ключу, поддержка и обновления проходят через продавца в Telegram.',
    },
    featuresBlock: {
      title: 'Возможности, которые делают работу со стримом проще.',
      intro:
        'SATURX фокусируется на практичных действиях: чат, заготовки, авто-отправка, аккаунты и отдельная вкладка viewer boost.',
      one: {
        title: 'Чат и заготовки',
        desc: 'Встроенный Kick-чат, подготовленные сообщения и быстрые действия помогают не терять темп во время трансляции.',
      },
      two: {
        title: 'Аккаунты и лицензия',
        desc: 'Рабочий процесс с несколькими аккаунтами и доступом по ключу собран в одном приложении.',
      },
      three: {
        title: 'Viewer boost controls',
        desc: 'Отдельная панель для настройки канала, цели и прокси без перехода в другой инструмент.',
      },
    },
    product: {
      title: 'Что такое SATURX',
      intro:
        'SATURX - это desktop software для Kick, который помогает управлять чатом, аккаунтами и активностью канала в одном интерфейсе.',
      whatTitle: 'Рабочее место для Kick-стрима',
      whatText:
        'Вместо набора вкладок и разрозненных инструментов SATURX собирает ключевые действия в одном дашборде: чат, стрим-превью, аккаунты, заготовки сообщений и controls для viewer boost.',
      forTitle: 'Для кого подходит',
      forItems: [
        'Стримерам Kick, которым нужен быстрый доступ к чату и управлению активностью.',
        'Модераторам и помощникам, которые работают с сообщениями, аккаунтами и рутиной канала.',
        'Командам, которым нужен понятный desktop workflow без лишних браузерных вкладок.',
      ],
      benefitsTitle: 'Ключевые преимущества',
      benefitsIntro: 'SATURX делает рабочий процесс понятнее: меньше ручной рутины, быстрее доступ к ключевым действиям.',
      benefits: [
        {
          title: 'Один чистый интерфейс',
          desc: 'Чат, стрим, аккаунты и настройки активности находятся в одном рабочем пространстве.',
        },
        {
          title: 'Быстрые сообщения',
          desc: 'Заготовки и авто-отправка помогают поддерживать темп без ручного повторения рутины.',
        },
        {
          title: 'Прозрачный доступ',
          desc: 'Лицензия, поддержка и обновления объяснены прямо на странице без лишних обещаний.',
        },
      ],
    },
    how: {
      title: 'Как это работает',
      intro: 'Процесс сделан простым: получить доступ, настроить канал и работать из одного окна.',
      steps: [
        {
          title: 'Получите доступ',
          desc: 'Свяжитесь через Telegram, уточните условия и получите архив приложения с лицензионным ключом.',
        },
        {
          title: 'Настройте рабочее место',
          desc: 'Введите ключ, укажите канал, подключите аккаунты и подготовьте сообщения или proxy-настройки.',
        },
        {
          title: 'Работайте во время стрима',
          desc: 'Используйте чат, стрим-превью, заготовки, авто-отправку и viewer controls из одного дашборда.',
        },
      ],
    },
    why: {
      title: 'Почему выбирают SATURX',
      intro: 'Главная идея продукта - меньше операционного шума и больше контроля над процессом.',
      reasons: [
        {
          title: 'Создан под Kick',
          desc: 'Тексты, интерфейс и сценарии сфокусированы на Kick-чате и задачах стримера.',
        },
        {
          title: 'Не перегружает экран',
          desc: 'Темная тема, компактные панели и понятная иерархия помогают быстро ориентироваться.',
        },
        {
          title: 'Поддержка рядом',
          desc: 'Вопросы по доступу, продлению и настройке решаются через Telegram.',
        },
      ],
    },
    trust: {
      title: 'Доступ, поддержка и обновления',
      intro: 'SATURX позиционируется как рабочий desktop-инструмент, поэтому важны прозрачность и аккуратное использование.',
      items: [
        {
          title: 'Доступ по ключу',
          desc: 'После покупки вы получаете лицензионный ключ и инструкцию по запуску.',
        },
        {
          title: 'Поддержка через Telegram',
          desc: 'Можно задать вопросы по установке, продлению, доступу и базовой настройке.',
        },
        {
          title: 'Без агрессивных обещаний',
          desc: 'Страница объясняет функциональность продукта без гарантии невозможных результатов.',
        },
      ],
    },
    faq: {
      title: 'FAQ',
      intro: 'Короткие ответы на вопросы, которые обычно возникают перед получением доступа.',
      items: [
        {
          question: 'SATURX - это интернет-магазин?',
          answer: 'Нет. Сейчас это лендинг desktop software для Kick, доступ к которому выдается по лицензии.',
        },
        {
          question: 'Какая платформа поддерживается?',
          answer: 'Продукт позиционируется как Windows desktop tool. Точные требования лучше уточнить перед покупкой.',
        },
        {
          question: 'Где получить поддержку?',
          answer: 'Основной канал поддержки и вопросов по доступу - Telegram.',
        },
        {
          question: 'Есть ли цена на сайте?',
          answer: 'Точной цены в проекте сейчас нет, поэтому сайт не публикует price schema и не обещает фиксированный тариф.',
        },
        {
          question: 'Можно ли индексировать русскую версию отдельно?',
          answer: 'Пока язык переключается в браузере. Для SEO лучше следующим этапом сделать отдельные URL /ru и /en.',
        },
      ],
    },
    screenshots: {
      title: 'Интерфейс SATURX',
      intro:
        'Скриншоты показывают реальный рабочий экран: чат, стрим-превью, аккаунты и отдельную вкладку viewer boost controls.',
      dashboardTitle: 'Dashboard: чат, стрим и аккаунты',
      dashboardDesc:
        'На скриншоте виден основной дашборд SATURX: Kick-чат, preview трансляции, список аккаунтов, поле сообщения и быстрые действия.',
      dashboardAlt:
        'Дашборд SATURX с Kick-чатом, превью стрима, списком аккаунтов, полем сообщения и controls активности',
      viewerbotTitle: 'Viewer boost controls',
      viewerbotDesc:
        'Отдельная вкладка показывает настройки канала, целевое число зрителей, proxy-поля и кнопки запуска или остановки.',
      viewerbotAlt:
        'Экран SATURX viewer boost controls с полем канала, настройкой цели, proxy-полями и кнопками запуска',
    },
    getSaturx: {
      title: 'Получить SATURX',
      intro:
        'Доступ к SATURX выдается по лицензии. Перед покупкой можно уточнить условия, совместимость и способ получения через Telegram.',
      accessTitle: 'Как получить доступ',
      accessText:
        'Напишите в Telegram, уточните детали и получите архив приложения, инструкцию и лицензионный ключ после оплаты.',
      includesTitle: 'Что входит',
      includes: [
        'Desktop dashboard для Kick-чата и стрим-превью.',
        'Работа с аккаунтами, заготовками сообщений и авто-отправкой.',
        'Viewer boost controls в отдельной вкладке.',
        'Инструкция, лицензионный ключ и поддержка по вопросам доступа.',
      ],
      supportTitle: 'Поддержка',
      supportText: 'Вопросы по доступу, продлению и базовой настройке решаются через Telegram.',
      safetyTitle: 'Аккуратное использование',
      safetyText:
        'SATURX описывает доступные controls и workflow. Используйте инструмент ответственно и соблюдайте правила платформы.',
      updatesTitle: 'Обновления',
      updatesText: 'Информация об актуальной версии, продлении ключа и изменениях доступна через канал поддержки.',
      telegram: 'Открыть Telegram',
      github: 'Открыть GitHub',
      footer: 'Для покупки, продления лицензии или поддержки используйте Telegram.',
    },
    footer: {
      product: 'Продукт',
      what: 'Обзор',
      features: 'Возможности',
      screenshots: 'Скриншоты',
      faq: 'FAQ',
      getSaturx: 'Получить SATURX',
      contact: 'Поддержка в Telegram',
      tagline: 'SATURX - desktop-инструмент для Kick-чата и дашборда стримеров и модераторов.',
    },
  },
  en: {
    nav: {
      home: 'Home',
      what: 'Product',
      features: 'Features',
      screenshots: 'Screenshots',
      faq: 'FAQ',
      getSaturx: 'Get SATURX',
    },
    cta: 'Get access',
    hero: {
      eyebrow: 'Desktop tool for Kick',
      headline: 'SATURX brings Kick chat, dashboard activity, and viewer controls into one workspace.',
      subheadline:
        'A desktop tool for Kick streamers and moderators with chat, accounts, message presets, auto-send workflows, and viewer boost controls in a clean interface.',
      desc: 'Less tab switching. More control. All in SATURX.',
      button: 'Get SATURX',
      learnMore: 'View Features',
      proof: ['Fast & Lightweight', 'Secure by design', 'Built for Kick'],
      visualTitle: 'Live dashboard preview',
      visualSubtitle: 'Chat, stream preview, accounts and activity controls in one workspace.',
      visualBadges: ['Chat', 'Accounts', 'Presets', 'Viewer controls'],
    },
    strip: {
      title:
        'SATURX keeps your Kick workflow in one place: chat, stream preview, accounts, presets, and activity tools without extra tab chaos.',
    },
    cards: {
      tag: 'Product',
      mainTitle: 'One dashboard instead of scattered tabs',
      mainDesc:
        'Manage Kick chat, accounts, prepared messages, and viewer boost controls from a single interface built for fast actions during a stream.',
      multiTitle: 'Multiple accounts',
      multiDesc: 'Switch accounts and send prepared messages from one working screen.',
      licenseTitle: 'License access',
      licenseDesc: 'Activation by key, support, and updates are handled through Telegram.',
    },
    featuresBlock: {
      title: 'Features that make stream work easier.',
      intro:
        'SATURX focuses on practical actions: chat, presets, auto-send, accounts, and a dedicated viewer boost controls tab.',
      one: {
        title: 'Chat and presets',
        desc: 'Embedded Kick chat, prepared messages, and quick actions help you keep momentum during the stream.',
      },
      two: {
        title: 'Accounts and license',
        desc: 'A multi-account workflow and key-based access are handled inside one desktop application.',
      },
      three: {
        title: 'Viewer boost controls',
        desc: 'A dedicated panel for channel, target, and proxy settings without moving to another tool.',
      },
    },
    product: {
      title: 'What is SATURX',
      intro:
        'SATURX is desktop software for Kick that helps manage chat, accounts, and channel activity from one clean interface.',
      whatTitle: 'A workspace for Kick streams',
      whatText:
        'Instead of scattered browser tabs and separate utilities, SATURX brings the main actions into one dashboard: chat, stream preview, accounts, message presets, and viewer boost controls.',
      forTitle: 'Who it is for',
      forItems: [
        'Kick streamers who want fast access to chat and activity controls.',
        'Moderators and assistants who handle messages, accounts, and channel routines.',
        'Teams that need a clear desktop workflow without juggling extra browser tabs.',
      ],
      benefitsTitle: 'Key benefits',
      benefitsIntro: 'SATURX makes the workflow easier to scan: less manual routine, faster access to the actions that matter.',
      benefits: [
        {
          title: 'One clean interface',
          desc: 'Chat, stream preview, accounts, and activity settings live in the same workspace.',
        },
        {
          title: 'Faster messaging',
          desc: 'Presets and auto-send workflows reduce repeated manual actions during a stream.',
        },
        {
          title: 'Clear access',
          desc: 'License access, support, and updates are explained directly without inflated claims.',
        },
      ],
    },
    how: {
      title: 'How it works',
      intro: 'The flow is simple: get access, set up your channel, and work from one desktop window.',
      steps: [
        {
          title: 'Get access',
          desc: 'Contact via Telegram, confirm the terms, and receive the app archive with a license key.',
        },
        {
          title: 'Set up your workspace',
          desc: 'Enter the key, choose your channel, connect accounts, and prepare messages or proxy settings.',
        },
        {
          title: 'Use it during the stream',
          desc: 'Run chat, stream preview, presets, auto-send workflows, and viewer controls from the dashboard.',
        },
      ],
    },
    why: {
      title: 'Why choose SATURX',
      intro: 'The product is built around less operational noise and more control over the streaming workflow.',
      reasons: [
        {
          title: 'Built around Kick',
          desc: 'The copy, interface, and workflows are focused on Kick chat and streamer tasks.',
        },
        {
          title: 'Less screen clutter',
          desc: 'A dark UI, compact panels, and clear hierarchy make the tool easier to scan.',
        },
        {
          title: 'Support nearby',
          desc: 'Questions about access, renewal, and setup are handled through Telegram.',
        },
      ],
    },
    trust: {
      title: 'Access, support, and updates',
      intro: 'SATURX is positioned as a working desktop tool, so clarity and responsible use matter.',
      items: [
        {
          title: 'Key-based access',
          desc: 'After purchase you receive a license key and launch instructions.',
        },
        {
          title: 'Telegram support',
          desc: 'Ask about installation, renewal, access, and basic setup through Telegram.',
        },
        {
          title: 'No aggressive promises',
          desc: 'The page explains the available workflow without guaranteeing impossible results.',
        },
      ],
    },
    faq: {
      title: 'FAQ',
      intro: 'Short answers to common questions before getting access.',
      items: [
        {
          question: 'Is SATURX an online store?',
          answer: 'No. This landing page presents desktop software for Kick with license-based access.',
        },
        {
          question: 'Which platform is supported?',
          answer: 'SATURX is positioned as a Windows desktop tool. Confirm exact requirements before purchase.',
        },
        {
          question: 'Where do I get support?',
          answer: 'The main support and access channel is Telegram.',
        },
        {
          question: 'Is there a price on the website?',
          answer: 'No exact price exists in the project yet, so the site does not publish price schema or fixed pricing claims.',
        },
        {
          question: 'Can the Russian version be indexed separately?',
          answer: 'For now, language switches in the browser. The next SEO step is separate /ru and /en URLs.',
        },
      ],
    },
    screenshots: {
      title: 'SATURX interface',
      intro:
        'Screenshots show the real working area: chat, stream preview, accounts, and a dedicated viewer boost controls tab.',
      dashboardTitle: 'Dashboard: chat, stream, and accounts',
      dashboardDesc:
        'The main SATURX dashboard shows Kick chat, stream preview, account list, message input, and quick activity controls.',
      dashboardAlt:
        'SATURX dashboard with Kick chat, stream preview, account list, message field, and activity controls',
      viewerbotTitle: 'Viewer boost controls',
      viewerbotDesc:
        'The viewer controls screen shows channel settings, target viewer count, proxy fields, and start or stop controls.',
      viewerbotAlt:
        'SATURX viewer boost controls screen with channel input, target viewer settings, proxy fields, and start controls',
    },
    getSaturx: {
      title: 'Get SATURX',
      intro:
        'SATURX access is provided by license. Before purchase, you can confirm terms, compatibility, and delivery through Telegram.',
      accessTitle: 'How to get access',
      accessText:
        'Message on Telegram, confirm the details, and receive the app archive, instructions, and license key after payment.',
      includesTitle: 'What is included',
      includes: [
        'Desktop dashboard for Kick chat and stream preview.',
        'Account workflow, message presets, and auto-send features.',
        'Viewer boost controls in a dedicated tab.',
        'Instructions, license key, and access-related support.',
      ],
      supportTitle: 'Support',
      supportText: 'Access, renewal, and basic setup questions are handled through Telegram.',
      safetyTitle: 'Responsible use',
      safetyText:
        'SATURX describes available controls and workflow. Use the tool responsibly and follow platform rules.',
      updatesTitle: 'Updates',
      updatesText: 'Current version details, key renewal, and change notes are available through the support channel.',
      telegram: 'Open Telegram',
      github: 'View GitHub',
      footer: 'For purchase, license renewal, or support, use Telegram.',
    },
    footer: {
      product: 'Product',
      what: 'Overview',
      features: 'Features',
      screenshots: 'Screenshots',
      faq: 'FAQ',
      getSaturx: 'Get SATURX',
      contact: 'Telegram support',
      tagline: 'SATURX - desktop Kick chat and dashboard tool for streamers and moderators.',
    },
  },
} as const

export type Translation = (typeof translations)[Lang]
