import jetbrains.buildServer.configs.kotlin.v2019_2.*
import jetbrains.buildServer.configs.kotlin.v2019_2.buildFeatures.commitStatusPublisher
import jetbrains.buildServer.configs.kotlin.v2019_2.buildSteps.script
import jetbrains.buildServer.configs.kotlin.v2019_2.triggers.vcs

version = "2020.1"

project {

    buildType(build)
    buildType(lint)
}

object build : BuildType({
    name = "make build"

    vcs {
        root(DslContext.settingsRoot)
    }

    steps {
        script {
            name = "make build"
            scriptContent = "make build"
        }
    }

    triggers {
        vcs {
            triggerRules = """
                +:refs/head/*
                +:refs/pull/*/merge
            """.trimIndent()
            branchFilter = ""
        }
    }

    features {
        pullRequests {
            vcsRootExtId = "${DslContext.settingsRoot.id}"
            provider = github {
                authType = token {
                    token = "credentialsJSON:f8789b57-76c2-4c9b-b8df-8de0d5cddf89"
                }
                filterAuthorRole = PullRequests.GitHubRoleFilter.EVERYBODY
            }
        }

        commitStatusPublisher {
            vcsRootExtId = "${DslContext.settingsRoot.id}"
            publisher = github {
                githubUrl = "https://api.github.com"
                authType = personalToken {
                    token = "credentialsJSON:f8789b57-76c2-4c9b-b8df-8de0d5cddf89"
                }
            }
        }
    }
})

object lint : BuildType({
    name = "make lint"

    vcs {
        root(DslContext.settingsRoot)
    }

    steps {
        script {
            name = "make lint"
            scriptContent = "make tools; make lint"
        }
    }

    triggers {
        vcs {
            triggerRules = """
                +:refs/head/*
                +:refs/pull/*/merge
            """.trimIndent()
            branchFilter = ""
        }
    }

    features {
            pullRequests {
                vcsRootExtId = "${DslContext.settingsRoot.id}"
                provider = github {
                    authType = token {
                        token = "credentialsJSON:f8789b57-76c2-4c9b-b8df-8de0d5cddf89"
                    }
                    filterAuthorRole = PullRequests.GitHubRoleFilter.EVERYBODY
                }
            }

            commitStatusPublisher {
                vcsRootExtId = "${DslContext.settingsRoot.id}"
                publisher = github {
                    githubUrl = "https://api.github.com"
                    authType = personalToken {
                        token = "credentialsJSON:f8789b57-76c2-4c9b-b8df-8de0d5cddf89"
                    }
                }
            }
        }
})
