# How to auto migrate database schema 

## what is `version`. how it works
Given a version number TABLE.INDEX.COLUMN, increment the:

- TABLE version when you add/drop any table,
- INDEX version when you add/drop/update and index
- COLUMN version when you add/drop/update any column

NB: current schema that is created by sql scripts is versioned as `0.0.0`. 

## How to add a new version

### Migrate table/column in gorm.AutoMigrate
 if migration works with gorm.AutoMigrate, please use it to migrate. It works without releasing new `Version` 
 - update your model
 - added your model in [AutoMigrate](migration.go#L63) if it doesn't exists
  ```
   db.AutoMigrate(&Migration{},&YourModel{})
  ```

### Migrate index/constraints manually if it is not easy to do in `AutoMigrate`
- create a new `Migration` with scripts
- append it in releases
```
var releases = []Migration{
	{
		Version:   "0.1.0",
		CreatedAt: time.Date(2021, 10, 15, 0, 0, 0, 0, time.UTC),
		Scripts: []string{
			"CREATE INDEX idx_allocation_path ON reference_objects (allocation_id,path);",
		},
	},
    {
		Version:   "0.1.1",
		CreatedAt: time.Date(2021, 10, 16, 0, 0, 0, 0, time.UTC),
		Scripts: []string{
			"sql1",
            "sql2",
		},
	},
}
```