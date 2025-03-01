import { Controller, Get, Post, Body, Patch, Param, Delete } from '@nestjs/common';
import { UserService } from './user.service';
import { CreateUserDto } from './dto/create-user.dto';
import { UpdateUserDto } from './dto/update-user.dto';
import { User, UserSchema } from './schemas/user.schema'
import { ApiOperation, ApiQuery, ApiResponse } from '@nestjs/swagger';

@Controller('user')
export class UserController {
  constructor(private readonly userService: UserService) {}

  @Post()

  @ApiOperation({
    summary: '创建单个用户',
    description: '用户详细信息创建单个用户'
  })
  @ApiResponse({
    status: 200,
    description: '返回用户信息',
    type: CreateUserDto
  })
  create(@Body() createUserDto: CreateUserDto) {
    return this.userService.create(createUserDto);
  }

  @Get(':id')
  @ApiOperation({
    summary: '获取单个用户信息',
    description: '获取单个用户详细信息'
  })
  @ApiResponse({
    status: 200,
    description: '单个用户信息',
    type: User
  })
  async findOne(@Param('id') id: string): Promise<{data: User}> {
    const data: User = await this.userService.findOne(id);
    return {data};
  }

  @Get()
  @ApiOperation({
    summary: '获取所有用户信息',
    description: '获取所有用户详细信息'
  })
  @ApiResponse({
    status: 200,
    description: '返回所有用户信息',
    type: User,
    isArray: true
  })
  async findAll(): Promise<{data: User[]}> {
    const data: User[] = await this.userService.findAll();
    return {data};
  }

  // @Patch(':id')
  // async update(@Param('id') id: string, @Body() updateUserDto: UpdateUserDto) {
  //   return await this.userService.update(id, updateUserDto);
  // }

}
