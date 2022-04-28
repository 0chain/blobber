# blobber development guide




## install `postgres:14` in docker as shared database for blobbers and validators

```
**********************************************
  Welcome to blobber/validator development CLI 
**********************************************

 
Please select which blobber/validator you will work on: 
1) 1
2) 2
3) 3
4) clean all
#? 1 

**********************************************
            Blobber/Validator 1
**********************************************
 
Please select what you will do: 
1) install postgres
2) start blobber
3) start validator
#? 1 
```


It will check and install a `blobber_postgres` container as shared database for all blobbers and validators, and initialized database `blobber_meta$i` and user `blobber_user$i`

## run local validator instance

```
**********************************************
  Welcome to blobber/validator development CLI 
**********************************************

 
Please select which blobber/validator you will work on: 
1) 1
2) 2
3) 3
4) clean all
#? 1

**********************************************
            Blobber/Validator 1
**********************************************
 
Please select what you will do: 
1) install postgres  3) start validator
2) start blobber     4) clean
#? 3

```


## run local blobber instance

```
**********************************************
  Welcome to blobber/validator development CLI 
**********************************************

 
Please select which blobber/validator you will work on: 
1) 1
2) 2
3) 3
4) clean all
#? 1

**********************************************
            Blobber/Validator 1
**********************************************
 
Please select what you will do: 
1) install postgres  3) start validator
2) start blobber     4) clean
#? 2

```