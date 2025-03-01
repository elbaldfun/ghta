import { IsString } from "class-validator";

export class UpdateCategoryDto {
    @IsString()
    readonly name: string

    @IsString()
    readonly parentId: string

    @IsString()
    readonly level: number

    @IsString()
    readonly path: string
}
