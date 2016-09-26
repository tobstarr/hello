#!/usr/bin/env groovy

node {
  stage "SCM"
  env.IMAGE_REPO = "127.0.0.1:30015/tobstarr/hello"

  checkout scm

  stage "Build"
  sh 'bash ./jenkins/build.sh'

  stage "Test"
  sh 'bash ./jenkins/test.sh'

  stage "Push"
  sh 'bash ./jenkins/push.sh'

  if (env.BRANCH_NAME == "master") {
    stage "Release"
    sh 'bash ./jenkins/release.sh'
  }
}
