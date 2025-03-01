// Primary Categories
db.categories.insertMany([
  {
    _id: ObjectId("64f5a321d89e3a0000000001"),
    name: "Application Software",
    description: "Top level category for Application Software",
    parentId: null,
    level: 1,
    path: "Application Software"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000002"), 
    name: "Development Tools",
    description: "Top level category for Development Tools",
    parentId: null,
    level: 1,
    path: "Development Tools"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000003"),
    name: "Programming Languages",
    description: "Top level category for Programming Languages",
    parentId: null,
    level: 1,
    path: "Programming Languages"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000004"),
    name: "System Tools",
    description: "Top level category for System Tools",
    parentId: null,
    level: 1,
    path: "System Tools"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000005"),
    name: "Artificial Intelligence",
    description: "Top level category for Artificial Intelligence",
    parentId: null,
    level: 1,
    path: "Artificial Intelligence"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000006"),
    name: "Blockchain",
    description: "Top level category for Blockchain",
    parentId: null,
    level: 1,
    path: "Blockchain"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000007"),
    name: "Security",
    description: "Top level category for Security",
    parentId: null,
    level: 1,
    path: "Security"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000008"),
    name: "Game Development",
    description: "Top level category for Game Development",
    parentId: null,
    level: 1,
    path: "Game Development"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000009"),
    name: "Data Processing",
    description: "Top level category for Data Processing",
    parentId: null,
    level: 1,
    path: "Data Processing"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000010"),
    name: "Education",
    description: "Top level category for Education",
    parentId: null,
    level: 1,
    path: "Education"
  }
]);

// Secondary Categories
db.categories.insertMany([
  {
    _id: ObjectId("64f5a321d89e3a0000000011"),
    name: "Desktop Applications",
    description: "Desktop applications under Application Software",
    parentId: ObjectId("64f5a321d89e3a0000000001"),
    level: 2,
    path: "Application Software/Desktop Applications"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000012"),
    name: "Mobile Applications",
    description: "Mobile applications under Application Software",
    parentId: ObjectId("64f5a321d89e3a0000000001"),
    level: 2,
    path: "Application Software/Mobile Applications"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000013"),
    name: "Web Applications",
    description: "Web applications under Application Software",
    parentId: ObjectId("64f5a321d89e3a0000000001"),
    level: 2,
    path: "Application Software/Web Applications"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000014"),
    name: "Source Code Management",
    description: "Source code management under Development Tools",
    parentId: ObjectId("64f5a321d89e3a0000000002"),
    level: 2,
    path: "Development Tools/Source Code Management"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000015"),
    name: "CICD",
    description: "CICD under Development Tools",
    parentId: ObjectId("64f5a321d89e3a0000000002"),
    level: 2,
    path: "Development Tools/CICD"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000016"),
    name: "Language Implementations",
    description: "Language implementations under Programming Languages",
    parentId: ObjectId("64f5a321d89e3a0000000003"),
    level: 2,
    path: "Programming Languages/Language Implementations"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000017"),
    name: "Language Ecosystem",
    description: "Language ecosystem under Programming Languages",
    parentId: ObjectId("64f5a321d89e3a0000000003"),
    level: 2,
    path: "Programming Languages/Language Ecosystem"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000018"),
    name: "Operating Systems",
    description: "Operating systems under System Tools",
    parentId: ObjectId("64f5a321d89e3a0000000004"),
    level: 2,
    path: "System Tools/Operating Systems"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000019"),
    name: "Containerization",
    description: "Containerization under System Tools",
    parentId: ObjectId("64f5a321d89e3a0000000004"),
    level: 2,
    path: "System Tools/Containerization"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000020"),
    name: "Network Tools",
    description: "Network tools under System Tools",
    parentId: ObjectId("64f5a321d89e3a0000000004"),
    level: 2,
    path: "System Tools/Network Tools"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000021"),
    name: "Machine Learning",
    description: "Machine learning under Artificial Intelligence",
    parentId: ObjectId("64f5a321d89e3a0000000005"),
    level: 2,
    path: "Artificial Intelligence/Machine Learning"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000022"),
    name: "Computer Vision",
    description: "Computer vision under Artificial Intelligence",
    parentId: ObjectId("64f5a321d89e3a0000000005"),
    level: 2,
    path: "Artificial Intelligence/Computer Vision"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000023"),
    name: "Natural Language Processing",
    description: "NLP under Artificial Intelligence",
    parentId: ObjectId("64f5a321d89e3a0000000005"),
    level: 2,
    path: "Artificial Intelligence/Natural Language Processing"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000024"),
    name: "Speech Recognition",
    description: "Speech recognition under Artificial Intelligence",
    parentId: ObjectId("64f5a321d89e3a0000000005"),
    level: 2,
    path: "Artificial Intelligence/Speech Recognition"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000025"),
    name: "Smart Contracts",
    description: "Smart contracts under Blockchain",
    parentId: ObjectId("64f5a321d89e3a0000000006"),
    level: 2,
    path: "Blockchain/Smart Contracts"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000026"),
    name: "Cryptography",
    description: "Cryptography under Blockchain",
    parentId: ObjectId("64f5a321d89e3a0000000006"),
    level: 2,
    path: "Blockchain/Cryptography"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000027"),
    name: "Penetration Testing",
    description: "Penetration testing under Security",
    parentId: ObjectId("64f5a321d89e3a0000000007"),
    level: 2,
    path: "Security/Penetration Testing"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000028"),
    name: "Network Security",
    description: "Network security under Security",
    parentId: ObjectId("64f5a321d89e3a0000000007"),
    level: 2,
    path: "Security/Network Security"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000029"),
    name: "Game Engines",
    description: "Game engines under Game Development",
    parentId: ObjectId("64f5a321d89e3a0000000008"),
    level: 2,
    path: "Game Development/Game Engines"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000030"),
    name: "Asset Management",
    description: "Asset management under Game Development",
    parentId: ObjectId("64f5a321d89e3a0000000008"),
    level: 2,
    path: "Game Development/Asset Management"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000031"),
    name: "Data Storage",
    description: "Data storage under Data Processing",
    parentId: ObjectId("64f5a321d89e3a0000000009"),
    level: 2,
    path: "Data Processing/Data Storage"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000032"),
    name: "ETL",
    description: "ETL under Data Processing",
    parentId: ObjectId("64f5a321d89e3a0000000009"),
    level: 2,
    path: "Data Processing/ETL"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000033"),
    name: "Learning Resources",
    description: "Learning resources under Education",
    parentId: ObjectId("64f5a321d89e3a0000000010"),
    level: 2,
    path: "Education/Learning Resources"
  }
]);

// Tertiary Categories
db.categories.insertMany([
  {
    _id: ObjectId("64f5a321d89e3a0000000034"),
    name: "Operating Systems",
    description: "Operating systems under Desktop Applications",
    parentId: ObjectId("64f5a321d89e3a0000000011"),
    level: 3,
    path: "Application Software/Desktop Applications/Operating Systems"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000035"),
    name: "Office Software",
    description: "Office software under Desktop Applications",
    parentId: ObjectId("64f5a321d89e3a0000000011"),
    level: 3,
    path: "Application Software/Desktop Applications/Office Software"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000036"),
    name: "Development Tools",
    description: "Development tools under Desktop Applications",
    parentId: ObjectId("64f5a321d89e3a0000000011"),
    level: 3,
    path: "Application Software/Desktop Applications/Development Tools"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000037"),
    name: "Multimedia",
    description: "Multimedia under Desktop Applications",
    parentId: ObjectId("64f5a321d89e3a0000000011"),
    level: 3,
    path: "Application Software/Desktop Applications/Multimedia"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000038"),
    name: "Android",
    description: "Android under Mobile Applications",
    parentId: ObjectId("64f5a321d89e3a0000000012"),
    level: 3,
    path: "Application Software/Mobile Applications/Android"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000039"),
    name: "iOS",
    description: "iOS under Mobile Applications",
    parentId: ObjectId("64f5a321d89e3a0000000012"),
    level: 3,
    path: "Application Software/Mobile Applications/iOS"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000040"),
    name: "CMS",
    description: "CMS under Web Applications",
    parentId: ObjectId("64f5a321d89e3a0000000013"),
    level: 3,
    path: "Application Software/Web Applications/CMS"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000041"),
    name: "E-commerce",
    description: "E-commerce under Web Applications",
    parentId: ObjectId("64f5a321d89e3a0000000013"),
    level: 3,
    path: "Application Software/Web Applications/E-commerce"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000042"),
    name: "Blog Platforms",
    description: "Blog platforms under Web Applications",
    parentId: ObjectId("64f5a321d89e3a0000000013"),
    level: 3,
    path: "Application Software/Web Applications/Blog Platforms"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000043"),
    name: "Version Control",
    description: "Version control under Source Code Management",
    parentId: ObjectId("64f5a321d89e3a0000000014"),
    level: 3,
    path: "Development Tools/Source Code Management/Version Control"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000044"),
    name: "Code Formatting",
    description: "Code formatting under Source Code Management",
    parentId: ObjectId("64f5a321d89e3a0000000014"),
    level: 3,
    path: "Development Tools/Source Code Management/Code Formatting"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000045"),
    name: "Build Tools",
    description: "Build tools under CICD",
    parentId: ObjectId("64f5a321d89e3a0000000015"),
    level: 3,
    path: "Development Tools/CICD/Build Tools"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000046"),
    name: "Deployment Tools",
    description: "Deployment tools under CICD",
    parentId: ObjectId("64f5a321d89e3a0000000015"),
    level: 3,
    path: "Development Tools/CICD/Deployment Tools"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000047"),
    name: "Interpreters",
    description: "Interpreters under Language Implementations",
    parentId: ObjectId("64f5a321d89e3a0000000016"),
    level: 3,
    path: "Programming Languages/Language Implementations/Interpreters"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000048"),
    name: "Compilers",
    description: "Compilers under Language Implementations",
    parentId: ObjectId("64f5a321d89e3a0000000016"),
    level: 3,
    path: "Programming Languages/Language Implementations/Compilers"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000049"),
    name: "Standard Libraries",
    description: "Standard libraries under Language Ecosystem",
    parentId: ObjectId("64f5a321d89e3a0000000017"),
    level: 3,
    path: "Programming Languages/Language Ecosystem/Standard Libraries"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000050"),
    name: "Third-party Libraries",
    description: "Third-party libraries under Language Ecosystem",
    parentId: ObjectId("64f5a321d89e3a0000000017"),
    level: 3,
    path: "Programming Languages/Language Ecosystem/Third-party Libraries"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000051"),
    name: "Kernels",
    description: "Kernels under Operating Systems",
    parentId: ObjectId("64f5a321d89e3a0000000018"),
    level: 3,
    path: "System Tools/Operating Systems/Kernels"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000052"),
    name: "Distributions",
    description: "Distributions under Operating Systems",
    parentId: ObjectId("64f5a321d89e3a0000000018"),
    level: 3,
    path: "System Tools/Operating Systems/Distributions"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000053"),
    name: "Container Engines",
    description: "Container engines under Containerization",
    parentId: ObjectId("64f5a321d89e3a0000000019"),
    level: 3,
    path: "System Tools/Containerization/Container Engines"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000054"),
    name: "Container Orchestration",
    description: "Container orchestration under Containerization",
    parentId: ObjectId("64f5a321d89e3a0000000019"),
    level: 3,
    path: "System Tools/Containerization/Container Orchestration"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000055"),
    name: "Proxies",
    description: "Proxies under Network Tools",
    parentId: ObjectId("64f5a321d89e3a0000000020"),
    level: 3,
    path: "System Tools/Network Tools/Proxies"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000056"),
    name: "Load Balancers",
    description: "Load balancers under Network Tools",
    parentId: ObjectId("64f5a321d89e3a0000000020"),
    level: 3,
    path: "System Tools/Network Tools/Load Balancers"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000057"),
    name: "Training Frameworks",
    description: "Training frameworks under Machine Learning",
    parentId: ObjectId("64f5a321d89e3a0000000021"),
    level: 3,
    path: "Artificial Intelligence/Machine Learning/Training Frameworks"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000058"),
    name: "Compute Optimization",
    description: "Compute optimization under Machine Learning",
    parentId: ObjectId("64f5a321d89e3a0000000021"),
    level: 3,
    path: "Artificial Intelligence/Machine Learning/Compute Optimization"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000059"),
    name: "Object Detection",
    description: "Object detection under Computer Vision",
    parentId: ObjectId("64f5a321d89e3a0000000022"),
    level: 3,
    path: "Artificial Intelligence/Computer Vision/Object Detection"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000060"),
    name: "OCR",
    description: "OCR under Computer Vision",
    parentId: ObjectId("64f5a321d89e3a0000000022"),
    level: 3,
    path: "Artificial Intelligence/Computer Vision/OCR"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000061"),
    name: "Pre-trained Models",
    description: "Pre-trained models under Natural Language Processing",
    parentId: ObjectId("64f5a321d89e3a0000000023"),
    level: 3,
    path: "Artificial Intelligence/Natural Language Processing/Pre-trained Models"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000062"),
    name: "TTS",
    description: "TTS under Speech Recognition",
    parentId: ObjectId("64f5a321d89e3a0000000024"),
    level: 3,
    path: "Artificial Intelligence/Speech Recognition/TTS"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000063"),
    name: "ASR",
    description: "ASR under Speech Recognition",
    parentId: ObjectId("64f5a321d89e3a0000000024"),
    level: 3,
    path: "Artificial Intelligence/Speech Recognition/ASR"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000064"),
    name: "Ethereum",
    description: "Ethereum under Smart Contracts",
    parentId: ObjectId("64f5a321d89e3a0000000025"),
    level: 3,
    path: "Blockchain/Smart Contracts/Ethereum"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000065"),
    name: "Other Chains",
    description: "Other chains under Smart Contracts",
    parentId: ObjectId("64f5a321d89e3a0000000025"),
    level: 3,
    path: "Blockchain/Smart Contracts/Other Chains"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000066"),
    name: "Zero-Knowledge Proofs",
    description: "Zero-knowledge proofs under Cryptography",
    parentId: ObjectId("64f5a321d89e3a0000000026"),
    level: 3,
    path: "Blockchain/Cryptography/Zero-Knowledge Proofs"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000067"),
    name: "Vulnerability Scanning",
    description: "Vulnerability scanning under Penetration Testing",
    parentId: ObjectId("64f5a321d89e3a0000000027"),
    level: 3,
    path: "Security/Penetration Testing/Vulnerability Scanning"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000068"),
    name: "Firewalls",
    description: "Firewalls under Network Security",
    parentId: ObjectId("64f5a321d89e3a0000000028"),
    level: 3,
    path: "Security/Network Security/Firewalls"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000069"),
    name: "Intrusion Detection",
    description: "Intrusion detection under Network Security",
    parentId: ObjectId("64f5a321d89e3a0000000028"),
    level: 3,
    path: "Security/Network Security/Intrusion Detection"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000070"),
    name: "2D Games",
    description: "2D games under Game Engines",
    parentId: ObjectId("64f5a321d89e3a0000000029"),
    level: 3,
    path: "Game Development/Game Engines/2D Games"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000071"),
    name: "3D Games",
    description: "3D games under Game Engines",
    parentId: ObjectId("64f5a321d89e3a0000000029"),
    level: 3,
    path: "Game Development/Game Engines/3D Games"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000072"),
    name: "Textures",
    description: "Textures under Asset Management",
    parentId: ObjectId("64f5a321d89e3a0000000030"),
    level: 3,
    path: "Game Development/Asset Management/Textures"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000073"),
    name: "Databases",
    description: "Databases under Data Storage",
    parentId: ObjectId("64f5a321d89e3a0000000031"),
    level: 3,
    path: "Data Processing/Data Storage/Databases"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000074"),
    name: "Data Extraction",
    description: "Data extraction under ETL",
    parentId: ObjectId("64f5a321d89e3a0000000032"),
    level: 3,
    path: "Data Processing/ETL/Data Extraction"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000075"),
    name: "Programming Learning",
    description: "Programming learning under Learning Resources",
    parentId: ObjectId("64f5a321d89e3a0000000033"),
    level: 3,
    path: "Education/Learning Resources/Programming Learning"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000076"),
    name: "Interactive Tutorials",
    description: "Interactive tutorials under Learning Resources",
    parentId: ObjectId("64f5a321d89e3a0000000033"),
    level: 3,
    path: "Education/Learning Resources/Interactive Tutorials"
  }
]);

// Quaternary Categories
db.categories.insertMany([
  {
    _id: ObjectId("64f5a321d89e3a0000000077"),
    name: "Linux Distributions",
    description: "Linux distributions under Operating Systems",
    parentId: ObjectId("64f5a321d89e3a0000000034"),
    level: 4,
    path: "Application Software/Desktop Applications/Operating Systems/Linux Distributions"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000078"),
    name: "BSD Systems",
    description: "BSD systems under Operating Systems",
    parentId: ObjectId("64f5a321d89e3a0000000034"),
    level: 4,
    path: "Application Software/Desktop Applications/Operating Systems/BSD Systems"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000079"),
    name: "Lightweight Systems",
    description: "Lightweight systems under Operating Systems",
    parentId: ObjectId("64f5a321d89e3a0000000034"),
    level: 4,
    path: "Application Software/Desktop Applications/Operating Systems/Lightweight Systems"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000080"),
    name: "Document Processing",
    description: "Document processing under Office Software",
    parentId: ObjectId("64f5a321d89e3a0000000035"),
    level: 4,
    path: "Application Software/Desktop Applications/Office Software/Document Processing"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000081"),
    name: "PDF Processing",
    description: "PDF processing under Office Software",
    parentId: ObjectId("64f5a321d89e3a0000000035"),
    level: 4,
    path: "Application Software/Desktop Applications/Office Software/PDF Processing"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000082"),
    name: "Task Management",
    description: "Task management under Office Software",
    parentId: ObjectId("64f5a321d89e3a0000000035"),
    level: 4,
    path: "Application Software/Desktop Applications/Office Software/Task Management"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000083"),
    name: "Code Editors",
    description: "Code editors under Development Tools",
    parentId: ObjectId("64f5a321d89e3a0000000036"),
    level: 4,
    path: "Application Software/Desktop Applications/Development Tools/Code Editors"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000084"),
    name: "Debugging Tools",
    description: "Debugging tools under Development Tools",
    parentId: ObjectId("64f5a321d89e3a0000000036"),
    level: 4,
    path: "Application Software/Desktop Applications/Development Tools/Debugging Tools"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000085"),
    name: "Code Comparison Tools",
    description: "Code comparison tools under Development Tools",
    parentId: ObjectId("64f5a321d89e3a0000000036"),
    level: 4,
    path: "Application Software/Desktop Applications/Development Tools/Code Comparison Tools"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000086"),
    name: "Audio Editing",
    description: "Audio editing under Multimedia",
    parentId: ObjectId("64f5a321d89e3a0000000037"),
    level: 4,
    path: "Application Software/Desktop Applications/Multimedia/Audio Editing"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000087"),
    name: "Video Editing",
    description: "Video editing under Multimedia",
    parentId: ObjectId("64f5a321d89e3a0000000037"),
    level: 4,
    path: "Application Software/Desktop Applications/Multimedia/Video Editing"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000088"),
    name: "3D Modeling",
    description: "3D modeling under Multimedia",
    parentId: ObjectId("64f5a321d89e3a0000000037"),
    level: 4,
    path: "Application Software/Desktop Applications/Multimedia/3D Modeling"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000089"),
    name: "UI Component Libraries",
    description: "UI component libraries under Android",
    parentId: ObjectId("64f5a321d89e3a0000000038"),
    level: 4,
    path: "Application Software/Mobile Applications/Android/UI Component Libraries"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000090"),
    name: "App Development",
    description: "App development under Android",
    parentId: ObjectId("64f5a321d89e3a0000000038"),
    level: 4,
    path: "Application Software/Mobile Applications/Android/App Development"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000091"),
    name: "Swift Frameworks",
    description: "Swift frameworks under iOS",
	parentId: ObjectId("64f5a321d89e3a0000000039"),
    level: 4,
    path: "Application Software/Mobile Applications/iOS/Swift Frameworks"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000092"),
    name: "iOS Applications",
    description: "iOS applications under iOS",
    parentId: ObjectId("64f5a321d89e3a0000000039"),
    level: 4,
    path: "Application Software/Mobile Applications/iOS/iOS Applications"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000093"),
    name: "Lightweight CMS",
    description: "Lightweight CMS under CMS",
    parentId: ObjectId("64f5a321d89e3a0000000040"),
    level: 4,
    path: "Application Software/Web Applications/CMS/Lightweight CMS"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000094"),
    name: "Traditional CMS",
    description: "Traditional CMS under CMS",
    parentId: ObjectId("64f5a321d89e3a0000000040"),
    level: 4,
    path: "Application Software/Web Applications/CMS/Traditional CMS"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000095"),
    name: "B2B",
    description: "B2B under E-commerce",
    parentId: ObjectId("64f5a321d89e3a0000000041"),
    level: 4,
    path: "Application Software/Web Applications/E-commerce/B2B"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000096"),
    name: "B2C",
    description: "B2C under E-commerce",
    parentId: ObjectId("64f5a321d89e3a0000000041"),
    level: 4,
    path: "Application Software/Web Applications/E-commerce/B2C"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000097"),
    name: "Order Management",
    description: "Order management under E-commerce",
    parentId: ObjectId("64f5a321d89e3a0000000041"),
    level: 4,
    path: "Application Software/Web Applications/E-commerce/Order Management"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000098"),
    name: "Static Blogs",
    description: "Static blogs under Blog Platforms",
    parentId: ObjectId("64f5a321d89e3a0000000042"),
    level: 4,
    path: "Application Software/Web Applications/Blog Platforms/Static Blogs"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000099"),
    name: "Dynamic Blogs",
    description: "Dynamic blogs under Blog Platforms",
    parentId: ObjectId("64f5a321d89e3a0000000042"),
    level: 4,
    path: "Application Software/Web Applications/Blog Platforms/Dynamic Blogs"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000100"),
    name: "Git-related",
    description: "Git-related under Version Control",
    parentId: ObjectId("64f5a321d89e3a0000000043"),
    level: 4,
    path: "Development Tools/Source Code Management/Version Control/Git-related"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000101"),
    name: "SVN-related",
    description: "SVN-related under Version Control",
    parentId: ObjectId("64f5a321d89e3a0000000043"),
    level: 4,
    path: "Development Tools/Source Code Management/Version Control/SVN-related"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000102"),
    name: "JavaScript",
    description: "JavaScript under Code Formatting",
    parentId: ObjectId("64f5a321d89e3a0000000044"),
    level: 4,
    path: "Development Tools/Source Code Management/Code Formatting/JavaScript"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000103"),
    name: "Python",
    description: "Python under Code Formatting",
    parentId: ObjectId("64f5a321d89e3a0000000044"),
    level: 4,
    path: "Development Tools/Source Code Management/Code Formatting/Python"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000104"),
    name: "Java",
    description: "Java under Code Formatting",
    parentId: ObjectId("64f5a321d89e3a0000000044"),
    level: 4,
    path: "Development Tools/Source Code Management/Code Formatting/Java"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000105"),
    name: "Java",
    description: "Java under Build Tools",
    parentId: ObjectId("64f5a321d89e3a0000000045"),
    level: 4,
    path: "Development Tools/CICD/Build Tools/Java"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000106"),
    name: "Node.js",
    description: "Node.js under Build Tools",
    parentId: ObjectId("64f5a321d89e3a0000000045"),
    level: 4,
    path: "Development Tools/CICD/Build Tools/Node.js"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000107"),
    name: "Python",
    description: "Python under Build Tools",
    parentId: ObjectId("64f5a321d89e3a0000000045"),
    level: 4,
    path: "Development Tools/CICD/Build Tools/Python"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000108"),
    name: "Container Deployment",
    description: "Container deployment under Deployment Tools",
    parentId: ObjectId("64f5a321d89e3a0000000046"),
    level: 4,
    path: "Development Tools/CICD/Deployment Tools/Container Deployment"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000109"),
    name: "Remote Deployment",
    description: "Remote deployment under Deployment Tools",
    parentId: ObjectId("64f5a321d89e3a0000000046"),
    level: 4,
    path: "Development Tools/CICD/Deployment Tools/Remote Deployment"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000110"),
    name: "Python",
    description: "Python under Interpreters",
    parentId: ObjectId("64f5a321d89e3a0000000047"),
    level: 4,
    path: "Programming Languages/Language Implementations/Interpreters/Python"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000111"),
    name: "JavaScript",
    description: "JavaScript under Interpreters",
    parentId: ObjectId("64f5a321d89e3a0000000047"),
    level: 4,
    path: "Programming Languages/Language Implementations/Interpreters/JavaScript"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000112"),
    name: "C/C++",
    description: "C/C++ under Compilers",
    parentId: ObjectId("64f5a321d89e3a0000000048"),
    level: 4,
    path: "Programming Languages/Language Implementations/Compilers/C/C++"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000113"),
    name: "Rust",
    description: "Rust under Compilers",
    parentId: ObjectId("64f5a321d89e3a0000000048"),
    level: 4,
    path: "Programming Languages/Language Implementations/Compilers/Rust"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000114"),
    name: "Java",
    description: "Java under Standard Libraries",
    parentId: ObjectId("64f5a321d89e3a0000000049"),
    level: 4,
    path: "Programming Languages/Language Ecosystem/Standard Libraries/Java"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000115"),
    name: "Python",
    description: "Python under Standard Libraries",
    parentId: ObjectId("64f5a321d89e3a0000000049"),
    level: 4,
    path: "Programming Languages/Language Ecosystem/Standard Libraries/Python"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000116"),
    name: "Deep Learning",
    description: "Deep learning under Third-party Libraries",
    parentId: ObjectId("64f5a321d89e3a0000000050"),
    level: 4,
    path: "Programming Languages/Language Ecosystem/Third-party Libraries/Deep Learning"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000117"),
    name: "Networking Libraries",
    description: "Networking libraries under Third-party Libraries",
    parentId: ObjectId("64f5a321d89e3a0000000050"),
    level: 4,
    path: "Programming Languages/Language Ecosystem/Third-party Libraries/Networking Libraries"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000118"),
    name: "Linux",
    description: "Linux under Kernels",
    parentId: ObjectId("64f5a321d89e3a0000000051"),
    level: 4,
    path: "System Tools/Operating Systems/Kernels/Linux"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000119"),
    name: "BSD",
    description: "BSD under Kernels",
    parentId: ObjectId("64f5a321d89e3a0000000051"),
    level: 4,
    path: "System Tools/Operating Systems/Kernels/BSD"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000120"),
    name: "Server OS",
    description: "Server OS under Distributions",
    parentId: ObjectId("64f5a321d89e3a0000000052"),
    level: 4,
    path: "System Tools/Operating Systems/Distributions/Server OS"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000121"),
    name: "Embedded OS",
    description: "Embedded OS under Distributions",
    parentId: ObjectId("64f5a321d89e3a0000000052"),
    level: 4,
    path: "System Tools/Operating Systems/Distributions/Embedded OS"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000122"),
    name: "Docker",
    description: "Docker under Container Engines",
    parentId: ObjectId("64f5a321d89e3a0000000053"),
    level: 4,
    path: "System Tools/Containerization/Container Engines/Docker"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000123"),
    name: "Podman",
    description: "Podman under Container Engines",
    parentId: ObjectId("64f5a321d89e3a0000000053"),
    level: 4,
    path: "System Tools/Containerization/Container Engines/Podman"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000124"),
    name: "Kubernetes",
    description: "Kubernetes under Container Orchestration",
    parentId: ObjectId("64f5a321d89e3a0000000054"),
    level: 4,
    path: "System Tools/Containerization/Container Orchestration/Kubernetes"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000125"),
    name: "VPN",
    description: "VPN under Proxies",
    parentId: ObjectId("64f5a321d89e3a0000000055"),
    level: 4,
    path: "System Tools/Network Tools/Proxies/VPN"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000126"),
    name: "HTTP Proxies",
    description: "HTTP proxies under Proxies",
    parentId: ObjectId("64f5a321d89e3a0000000055"),
    level: 4,
    path: "System Tools/Network Tools/Proxies/HTTP Proxies"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000127"),
    name: "Reverse Proxies",
    description: "Reverse proxies under Load Balancers",
    parentId: ObjectId("64f5a321d89e3a0000000056"),
    level: 4,
    path: "System Tools/Network Tools/Load Balancers/Reverse Proxies"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000128"),
    name: "Python",
    description: "Python under Training Frameworks",
    parentId: ObjectId("64f5a321d89e3a0000000057"),
    level: 4,
    path: "Artificial Intelligence/Machine Learning/Training Frameworks/Python"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000129"),
    name: "C++",
    description: "C++ under Training Frameworks",
    parentId: ObjectId("64f5a321d89e3a0000000057"),
    level: 4,
    path: "Artificial Intelligence/Machine Learning/Training Frameworks/C++"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000130"),
    name: "GPU",
    description: "GPU under Compute Optimization",
    parentId: ObjectId("64f5a321d89e3a0000000058"),
    level: 4,
    path: "Artificial Intelligence/Machine Learning/Compute Optimization/GPU"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000131"),
    name: "ONNX",
    description: "ONNX under Compute Optimization",
    parentId: ObjectId("64f5a321d89e3a0000000058"),
    level: 4,
    path: "Artificial Intelligence/Machine Learning/Compute Optimization/ONNX"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000132"),
    name: "YOLO",
    description: "YOLO under Object Detection",
    parentId: ObjectId("64f5a321d89e3a0000000059"),
    level: 4,
    path: "Artificial Intelligence/Computer Vision/Object Detection/YOLO"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000133"),
    name: "Faster R-CNN",
    description: "Faster R-CNN under Object Detection",
    parentId: ObjectId("64f5a321d89e3a0000000059"),
    level: 4,
    path: "Artificial Intelligence/Computer Vision/Object Detection/Faster R-CNN"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000134"),
    name: "Tesseract",
    description: "Tesseract under OCR",
    parentId: ObjectId("64f5a321d89e3a0000000060"),
    level: 4,
    path: "Artificial Intelligence/Computer Vision/OCR/Tesseract"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000135"),
    name: "GPT",
    description: "GPT under Pre-trained Models",
    parentId: ObjectId("64f5a321d89e3a0000000061"),
    level: 4,
    path: "Artificial Intelligence/Natural Language Processing/Pre-trained Models/GPT"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000136"),
    name: "BERT",
    description: "BERT under Pre-trained Models",
    parentId: ObjectId("64f5a321d89e3a0000000061"),
    level: 4,
    path: "Artificial Intelligence/Natural Language Processing/Pre-trained Models/BERT"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000137"),
    name: "Tacotron",
    description: "Tacotron under TTS",
    parentId: ObjectId("64f5a321d89e3a0000000062"),
    level: 4,
    path: "Artificial Intelligence/Speech Recognition/TTS/Tacotron"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000138"),
    name: "Whisper",
    description: "Whisper under ASR",
    parentId: ObjectId("64f5a321d89e3a0000000063"),
    level: 4,
    path: "Artificial Intelligence/Speech Recognition/ASR/Whisper"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000139"),
    name: "Solidity",
    description: "Solidity under Ethereum",
    parentId: ObjectId("64f5a321d89e3a0000000064"),
    level: 4,
    path: "Blockchain/Smart Contracts/Ethereum/Solidity"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000140"),
    name: "Solana",
    description: "Solana under Other Chains",
    parentId: ObjectId("64f5a321d89e3a0000000065"),
    level: 4,
    path: "Blockchain/Smart Contracts/Other Chains/Solana"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000141"),
    name: "ZKP",
    description: "ZKP under Zero-Knowledge Proofs",
    parentId: ObjectId("64f5a321d89e3a0000000066"),
    level: 4,
    path: "Blockchain/Cryptography/Zero-Knowledge Proofs/ZKP"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000142"),
    name: "Web",
    description: "Web under Vulnerability Scanning",
    parentId: ObjectId("64f5a321d89e3a0000000067"),
    level: 4,
    path: "Security/Penetration Testing/Vulnerability Scanning/Web"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000143"),
    name: "Network",
    description: "Network under Vulnerability Scanning",
    parentId: ObjectId("64f5a321d89e3a0000000067"),
    level: 4,
    path: "Security/Penetration Testing/Vulnerability Scanning/Network"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000144"),
    name: "Layer 7 Firewalls",
    description: "Layer 7 firewalls under Firewalls",
    parentId: ObjectId("64f5a321d89e3a0000000068"),
    level: 4,
    path: "Security/Network Security/Firewalls/Layer 7 Firewalls"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000145"),
    name: "IDS/IPS",
    description: "IDS/IPS under Intrusion Detection",
    parentId: ObjectId("64f5a321d89e3a0000000069"),
    level: 4,
    path: "Security/Network Security/Intrusion Detection/IDS/IPS"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000146"),
    name: "Cocos2d",
    description: "Cocos2d under 2D Games",
    parentId: ObjectId("64f5a321d89e3a0000000070"),
    level: 4,
    path: "Game Development/Game Engines/2D Games/Cocos2d"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000147"),
    name: "Unity",
    description: "Unity under 3D Games",
    parentId: ObjectId("64f5a321d89e3a0000000071"),
    level: 4,
    path: "Game Development/Game Engines/3D Games/Unity"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000148"),
    name: "PBR Textures",
    description: "PBR textures under Textures",
    parentId: ObjectId("64f5a321d89e3a0000000072"),
    level: 4,
    path: "Game Development/Asset Management/Textures/PBR Textures"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000149"),
    name: "Relational",
    description: "Relational under Databases",
    parentId: ObjectId("64f5a321d89e3a0000000073"),
    level: 4,
    path: "Data Processing/Data Storage/Databases/Relational"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000150"),
    name: "NoSQL",
    description: "NoSQL under Databases",
    parentId: ObjectId("64f5a321d89e3a0000000073"),
    level: 4,
    path: "Data Processing/Data Storage/Databases/NoSQL"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000151"),
    name: "Stream Processing",
    description: "Stream processing under Data Extraction",
    parentId: ObjectId("64f5a321d89e3a0000000074"),
    level: 4,
    path: "Data Processing/ETL/Data Extraction/Stream Processing"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000152"),
    name: "Batch Processing",
    description: "Batch processing under Data Extraction",
    parentId: ObjectId("64f5a321d89e3a0000000074"),
    level: 4,
    path: "Data Processing/ETL/Data Extraction/Batch Processing"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000153"),
    name: "Computer Fundamentals",
    description: "Computer fundamentals under Programming Learning",
    parentId: ObjectId("64f5a321d89e3a0000000075"),
    level: 4,
    path: "Education/Learning Resources/Programming Learning/Computer Fundamentals"
  },
  {
    _id: ObjectId("64f5a321d89e3a0000000154"),
    name: "Code Learning",
    description: "Code learning under Interactive Tutorials",
    parentId: ObjectId("64f5a321d89e3a0000000076"),
    level: 4,
    path: "Education/Learning Resources/Interactive Tutorials/Code Learning"
  }
]);