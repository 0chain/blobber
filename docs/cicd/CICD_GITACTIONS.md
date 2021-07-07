

## Guide to CI/CD using github actions
  <!-- Details of CI/CD setup using github -->
## Workflow Creation.
 - A new workflow is created using Go project with the file name called "build.yml".
 - By default the path of build.yml is ".github/workflows.build.yml"
 - Completed or running CI/CD can be seen under actions option.


## Details of components being used in build.yml.
#### Workflow name
Here the name of the workflow is defined i.e. "Dockerize"
```
name: Dockerize
```

#### Input Option to trigger manually builds
To run the workflow using manual option, *work_dispatch* is used. Which will ask for the input to tigger the builds with *latest* tag or not. If we select for **yes**, image will be build with *latest* tag as well as with *branch-commitid* tag. But if we select for **no**, image will be build with *branch-commitid* tag only.

```
on:
  workflow_dispatch:
    inputs:
      latest_tag:
        description: 'type yes for building latest tag'
        default: 'no'
        required: true
```

#### Global ENV setup
Environment variable is defined with the secrets added to the repository. Here secrets contains the docker images(example- dockerhub) repository name.
```
env:
  BLOBBER_REGISTRY: ${{ secrets.BLOBBER_REGISTRY }}
  VALIDATOR_REGISTRY: ${{ secrets.VALIDATOR_REGISTRY }}
```

#### Defining jobs and runner
Jobs are defined which contains the various steps for creating and pushing the builds. Runner envionment is also defined used for making the builds.
```
jobs:
    dockerize_blobber:
       runs-on: ubuntu-20.04
    ...
    dockerize_validator:
        runs-on: ubuntu-20.04
```

#### Different steps used in creating the builds
Here different steps are defined used for creating the builds.
 - *uses* --> checkout to branch from what code to create the builds.
 - *Get the version* --> Creating the tags by combining the branch name & first 8 digits of commit id.
 - *Login to Docker Hub* --> Logging into the docker hub using Username and Password from secrets of the repository.
 - *Build blobber/validator* --> Building, tagging and pushing the docker images with the *Get the version* tag.
 - *Push blobber/validator* --> Here we are checking if the input given by user is **yes**, images is also pushed with latest tag also.

For Blobber
```
steps:
- uses: actions/checkout@v2

- name: Get the version
    id: get_version
    run: |
    BRANCH=$(echo ${GITHUB_REF#refs/heads/} | sed 's/\//-/g')
    SHORT_SHA=$(echo $GITHUB_SHA | head -c 8)
    echo ::set-output name=BRANCH::${BRANCH}
    echo ::set-output name=VERSION::${BRANCH}-${SHORT_SHA} 

- name: Login to Docker Hub
    uses: docker/login-action@v1
    with:
    username: ${{ secrets.DOCKERHUB_USERNAME }}
    password: ${{ secrets.DOCKERHUB_PASSWORD }}

- name: Build blobber
    run: |
    docker build -t $BLOBBER_REGISTRY:$TAG -f "$DOCKERFILE_BLOB" .
    docker tag $BLOBBER_REGISTRY:$TAG $BLOBBER_REGISTRY:latest
    ocker push $BLOBBER_REGISTRY:$TAG
    env:
    TAG: ${{ steps.get_version.outputs.VERSION }}
    DOCKERFILE_BLOB: "docker.local/Dockerfile"

- name: Push blobber
    run: |
    if [[ "$PUSH_LATEST" == "yes" ]]; then
        docker push $BLOBBER_REGISTRY:latest
    fi
    env:
    PUSH_LATEST: ${{ github.event.inputs.latest_tag }}
```
For Validator
```
steps:
- uses: actions/checkout@v1

- name: Get the version
    id: get_version
    run: |
    BRANCH=$(echo ${GITHUB_REF#refs/heads/} | sed 's/\//-/g')
    SHORT_SHA=$(echo $GITHUB_SHA | head -c 8)
    echo ::set-output name=BRANCH::${BRANCH}
    echo ::set-output name=VERSION::${BRANCH}-${SHORT_SHA}    
- name: Login to Docker Hub
    uses: docker/login-action@v1
    with:
    username: ${{ secrets.DOCKERHUB_USERNAME }}
    password: ${{ secrets.DOCKERHUB_PASSWORD }}

- name: Build validator
    run: |
    docker build -t $VALIDATOR_REGISTRY:$TAG -f "$DOCKERFILE_PROXY" .
    docker tag $VALIDATOR_REGISTRY:$TAG $VALIDATOR_REGISTRY:latest
    docker push $VALIDATOR_REGISTRY:$TAG
    env:
    TAG: ${{ steps.get_version.outputs.VERSION }}
    DOCKERFILE_PROXY: "docker.local/ValidatorDockerfile"

- name: Push validator
    run: |
    if [[ "$PUSH_LATEST" == "yes" ]]; then
        docker push $VALIDATOR_REGISTRY:latest
    fi
    env:
    PUSH_LATEST: ${{ github.event.inputs.latest_tag }}
```
