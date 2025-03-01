import { Injectable } from '@nestjs/common';
import { InjectModel } from '@nestjs/mongoose';
import { Model } from 'mongoose';
import { Category } from './schemas/category.schema';   
import { UpdateCategoryDto } from './dto/update-category.dto';
import { CreateCategoryDto } from './dto/create-category.dto';

export interface CategoryTree {
  id: string;
  name: string;
  path: string;
  children?: CategoryTree[];
}

@Injectable()
export class CategoryService {

    constructor(
        @InjectModel(Category.name) private readonly categoryModel: Model<Category>
    ) {}

    async create(createCategoryDto: CreateCategoryDto): Promise<Category> {
        return this.categoryModel.create(createCategoryDto);
    }

    async findAll(): Promise<CategoryTree[]> {
        // 1. 获取所有分类
        const categories: Category[] = await this.categoryModel.find();
        
        // 2. 构建树形结构
        return this.buildCategoryTree(categories);
    }

    private buildCategoryTree(categories: Category[], parentId: string | null = null): CategoryTree[] {
        const tree: CategoryTree[] = [];

        // 找出当前层级的所有分类
        const currentLevelCategories = categories.filter(cat => 
            cat.parentId?.toString() === parentId || (parentId === null && cat.parentId === null)
        );

        // 递归构建树
        for (const category of currentLevelCategories) {
            const node: CategoryTree = {
                id: category._id.toString(),
                name: category.name,
                path: category.path,
            };

            // 查找子分类
            const children = this.buildCategoryTree(categories, category._id.toString());
            
            // 如果有子分类，则添加children属性
            if (children.length > 0) {
                node.children = children;
            }

            tree.push(node);
        }

        return tree;
    }

    async findOne(id: string): Promise<Category> {
        return this.categoryModel.findById(id);
    }

    async update(id: string, updateCategoryDto: UpdateCategoryDto): Promise<Category> {
        return this.categoryModel.findByIdAndUpdate({ _id: id }, updateCategoryDto).exec()
    }

    async remove(id: string): Promise<Category> {
        return this.categoryModel.findByIdAndDelete({ _id: id }).exec();
    }
}
