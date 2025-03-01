import { Injectable } from '@nestjs/common';
import { InjectModel } from '@nestjs/mongoose';
import { Model } from 'mongoose';
import { Category } from './schemas/category.schema';   
import { UpdateCategoryDto } from './dto/update-category.dto';
import { CreateCategoryDto } from './dto/create-category.dto';

@Injectable()
export class CategoryService {

    constructor(
        @InjectModel(Category.name) private readonly categoryModel: Model<Category>
    ) {}

    async create(createCategoryDto: CreateCategoryDto): Promise<Category> {
        return this.categoryModel.create(createCategoryDto);
    }

    async findAll(): Promise<Category[]> {
        return this.categoryModel.find();
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
