import { IsString } from "class-validator";

export class CreateUserDto {
    @IsString()
    readonly id: string;

    @IsString()
    readonly name: string
}
