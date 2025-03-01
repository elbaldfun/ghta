import { Controller, Post, Get, Body, Param, Patch, Delete } from '@nestjs/common';
import { CategoryService } from './category.service';
import { CreateCategoryDto } from './dto/create-category.dto';
import { UpdateCategoryDto } from './dto/update-category.dto';
import { ApiOperation, ApiResponse } from '@nestjs/swagger';

@Controller('category')
export class CategoryController {
    constructor(private readonly categoryService: CategoryService) {}

    @Post()
    @ApiOperation({
        summary: '创建分类',
        description: '创建分类'
    })
    @ApiResponse({
        status: 200,
        description: '创建分类成功'
    })
    async create(@Body() createCategoryDto: CreateCategoryDto) {
        const data = await this.categoryService.create(createCategoryDto);
        return {data};
    }

    @Get()
    @ApiOperation({
        summary: '获取所有分类',
        description: '获取所有分类'
    })
    @ApiResponse({
        status: 200,
        description: '返回所有分类'
    })
    async findAll() {
        const data = await this.categoryService.findAll();
        return {data};
    }

    @Get(':id')
    @ApiOperation({
        summary: '获取单个分类',
        description: '获取单个分类'
    })
    @ApiResponse({
        status: 200,
        description: '返回单个分类'
    })
    async findOne(@Param('id') id: string) {
        const data = await this.categoryService.findOne(id);
        return {data};
    }

    @Patch(':id')
    @ApiOperation({
        summary: '更新分类',
        description: '更新分类'
    })
    @ApiResponse({
        status: 200,
        description: '更新分类成功'
    })
    async update(@Param('id') id: string, @Body() updateCategoryDto: UpdateCategoryDto) {
        const data = await this.categoryService.update(id, updateCategoryDto);
        return {data};
    }

    @Delete(':id')
    @ApiOperation({
        summary: '删除分类',
        description: '删除分类'
    })
    @ApiResponse({
        status: 200,
        description: '删除分类成功'
    })
    async remove(@Param('id') id: string) {
        const data = await this.categoryService.remove(id);
        return {data};
    }
    
}