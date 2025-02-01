import { IsDateString, IsEnum, IsString } from "class-validator";

export enum LoginType {
    GOOGLE = 'google',
    GITHUB = 'github',
}

export class UserDto {
    @IsString()
    readonly id: string;

    @IsString()
    readonly name: string

    @IsString()
    readonly email: string

    @IsEnum(LoginType)
    readonly loginType: string

    @IsDateString()
    readonly createdAt: string

    @IsDateString()
    readonly updatedAt: string
}
