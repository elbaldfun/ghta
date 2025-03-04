export interface ICategoryAnalysisResult {
  categoryId: string;
  path: string;
  isNewCategory?: boolean;
  newPath?: string;
}

export interface IGitHubRepo {
  id: string;
  name: string;
  description: string;
  language: string;
  topics: string[];
  // 其他相关字段...
}

export interface ICategory {
  _id: string;
  name: string;
  path: string;
  parentId?: string;
  level: number;
  // 其他相关字段...
} 