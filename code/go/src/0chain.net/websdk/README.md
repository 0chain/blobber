# SDK using WebAssembly(WASM)

## Table of Contents
- [SDK using WebAssembly(WASM)](#sdk-using-webassemblywasm)
    - [Table of Contents](#table-of-contents)
    - [Building SDK](#building-sdk)
    - [SDK Usage](#sdk-usage)
    - [Testing Web SDK](#testing-web-sdk)
    - [Testing Electron SDK](#testing-electron-sdk)
    - [Deleting SDK directories](#deleting-sdk-directories)
---
## Building SDK
Run below comand to generate **SDK**. SDK for web will be under 
```
make sdk
```
---
## SDK Usage
![Alt text:](websdk.png "SDK Usage:")
---
## Testing Web SDK
Run fileserver using below commands and open any browser and go to url http://localhost:8080
```
cd 0chainwebsdk
./fileserver.exe
```
---
## Testing Electron SDK
Run below commands to launch the app
```
cd 0chainelectronsdk
npm install
npm start
```
---
## Deleting SDK directories
```
make clean
```