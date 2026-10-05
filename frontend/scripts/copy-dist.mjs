import { cpSync, mkdirSync, rmSync } from 'node:fs'
import { resolve } from 'node:path'

const source = resolve('.output/public')
const destination = resolve('dist')

rmSync(destination, { recursive: true, force: true })
mkdirSync(destination, { recursive: true })
cpSync(source, destination, { recursive: true })
