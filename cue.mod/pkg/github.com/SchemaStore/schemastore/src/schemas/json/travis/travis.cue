package travis

import (
	"list"
	"strings"
)

#Travis: {
	// JSON schema for Travis CI configuration files
	@jsonschema(schema="http://json-schema.org/draft-04/schema#")
	#job & {
		notifications?: {
			webhooks?: #possiblySecretStringOrPossiblySecretStringTypeArrayUnique | bool | #notificationObject.webhooks | list.UniqueItems() & [_, ...] & [...#notificationObject.webhooks]
			slack?:    #slackRoom | bool | #notificationObject.slack | list.UniqueItems() & [_, ...] & [...#notificationObject.slack]
			email?:    #possiblySecretStringOrPossiblySecretStringTypeArrayUnique | bool | #notificationObject.email | list.UniqueItems() & [_, ...] & [...#notificationObject.email]
			irc?:      #possiblySecretStringOrPossiblySecretStringTypeArrayUnique | bool | #notificationObject.irc | list.UniqueItems() & [_, ...] & [...#notificationObject.irc]
			pushover?: #nonEmptyStringOrArrayOfNonEmptyStrings | bool | #notificationObject.pushover | list.UniqueItems() & [_, ...] & [...#notificationObject.pushover]
			campfire?: #possiblySecretStringOrPossiblySecretStringTypeArrayUnique | bool | #notificationObject.campfire | list.UniqueItems() & [_, ...] & [...#notificationObject.campfire]
			flowdock?: #possiblySecretString | bool | #notificationObject.flowdock | list.UniqueItems() & [_, ...] & [...#notificationObject.flowdock]
			hipchat?:  #possiblySecretStringOrPossiblySecretStringTypeArrayUnique | bool | #notificationObject.hipchat | list.UniqueItems() & [_, ...] & [...#notificationObject.hipchat]
		}
		matrix?: {
			exclude?: [...#job]
			include?: [...#job]
			allow_failures?: [...#job]

			// If some rows in the build matrix are allowed to fail, the build
			// won’t be marked as finished until they have completed. To mark
			// the build as finished as soon as possible, add fast_finish:
			// true
			fast_finish?: bool
		}
		jobs?: {
			include?: [...#job & {
				// The name of the build stage
				stage?: string | *"test"
				...
			}]
			exclude?: [...#job & {
				// The name of the build stage
				stage?: string | *"test"
				...
			}]
			allow_failures?: [...#job]

			// If some rows in the build matrix are allowed to fail, the build
			// won’t be marked as finished until they have completed. To mark
			// the build as finished as soon as possible, add fast_finish:
			// true
			fast_finish?: bool
		}

		// Specifies the order of build stages
		stages?: [...string | {
			name?: string

			// Specifies a condition for the stage
			if?: string
		}]

		// Build config specification version
		version?: =~"^(~>|>|>=|=|<=|<) (\\d+(?:\\.\\d+)?(?:\\.\\d+)?)$"

		// Import YAML config snippets that can be shared across
		// repositories.
		import?: list.UniqueItems() & [...#import] | #import
		...
	}

	#nonEmptyString: strings.MinRunes(1)

	#notRequiredNonEmptyString: #nonEmptyString | null

	#arrayOfNonEmptyStrings: [...#nonEmptyString]

	#nonEmptyStringOrArrayOfNonEmptyStrings: #nonEmptyString | #arrayOfNonEmptyStrings

	#notRequiredNonEmptyStringOrArrayOfNonEmptyStrings: #nonEmptyStringOrArrayOfNonEmptyStrings | null

	#stringArrayUnique: list.UniqueItems() & [_, ...] & [...#nonEmptyString]

	#stringOrStringArrayUnique: #nonEmptyString | #stringArrayUnique

	#stringOrNumber: #nonEmptyString | number

	#stringOrNumberAndBothAreTypeArrayUnique: list.UniqueItems() & [_, ...] & [...#stringOrNumber]

	#stringOrNumberOrAcceptBothTypeAsArrayUnique: #stringOrNumber | #stringOrNumberAndBothAreTypeArrayUnique

	#secretString: secure?: #nonEmptyString

	#possiblySecretString: string | {
		secure?: string
	}

	#possiblySecretStringOrPossiblySecretStringTypeArrayUnique: #possiblySecretString | list.UniqueItems() & [_, ...] & [...#possiblySecretString]

	#slackRoom: =~".+:.+(#.+)?" | #secretString

	#notificationFrequency: "always" | "never" | "change"

	#step: (bool | ("skip" | "ignore") | string | [...string]) & (bool | string | [...])

	#service: "cassandra" | "couchdb" | "docker" | "elasticsearch" | "mariadb" | "memcached" | "mongodb" | "mysql" | "neo4j" | "postgresql" | "rabbitmq" | "redis" | "rethinkdb" | "riak" | "xvfb"

	#cache: "apt" | "bundler" | "cargo" | "ccache" | "cocoapods" | "packages" | "pip" | "yarn" | "npm"

	#xcodeVersions: "xcode6.4" | "xcode7.3" | "xcode8" | "xcode8.3" | "xcode9" | "xcode9.1" | "xcode9.2" | "xcode9.3" | "xcode9.4" | "xcode10" | "xcode10.1" | "xcode10.2" | "xcode10.3" | "xcode11" | "xcode11.1" | "xcode11.2" | "xcode11.3" | "xcode11.4" | "xcode11.5" | "xcode11.6" | "xcode11.7" | "xcode12u" | "xcode12" | "xcode12.1" | "xcode12.2" | "xcode12.3"

	#envVars: #envVar | [...#envVar]

	#envVar: =~"[^=]+=.*" | {
		secure?: =~"[^=]+=.*"
	}

	#job: {
		language?:     "android" | "bash" | "c" | "c++" | "clojure" | "cpp" | "crystal" | "csharp" | "d" | "dart" | "dartlang" | "elixir" | "elm" | "erlang" | "generic" | "go" | "golang" | "groovy" | "haskell" | "haxe" | "java" | "javascript" | "julia" | "jvm" | "matlab" | "minimal" | "nix" | "node" | "node.js" | "node_js" | "nodejs" | "obj-c" | "obj_c" | "objective-c" | "objective_c" | "perl" | "perl6" | "php" | "python" | "r" | "ruby" | "rust" | "scala" | "sh" | "shell" | "smalltalk"
		matlab?:       #stringOrStringArrayUnique
		elm?:          #stringOrStringArrayUnique
		"elm-test"?:   #nonEmptyString
		"elm-format"?: #nonEmptyString
		haxe?: [...string]
		scala?: [...string]
		sbt_args?: string
		crystal?: [...string]
		neko?: string
		hxml?: [...string]
		smalltalk?: [...string]
		perl?: [...string]
		perl6?: [...string]
		d?: [...string]
		dart?: [...string]
		dart_task?: [...{
			test?:            string
			install_dartium?: bool
			xvfb?:            bool
			dartanalyzer?:    bool
			dartfmt?:         bool
			...
		}]
		ghc?: [...string]
		lein?: string
		android?: {
			components?: [...string]
			licenses?: [...string]
		}
		node_js?:  #stringOrNumberOrAcceptBothTypeAsArrayUnique
		compiler?: [..."clang" | "gcc"] | ("clang" | "gcc")
		php?:      #stringOrNumberOrAcceptBothTypeAsArrayUnique
		go?:       [...string] | string
		jdk?:      string | [...string]

		// When the optional solution key is present, Travis will run
		// NuGet package restore and build the given solution.
		solution?:        string
		mono?:            "none" | [...string]
		xcode_project?:   string
		xcode_workspace?: string
		xcode_scheme?:    string
		xcode_sdk?:       string

		// By default, Travis CI will assume that your Podfile is in the
		// root of the repository. If this is not the case, you can
		// specify where the Podfile is
		podfile?:        string
		python?:         #stringOrNumberOrAcceptBothTypeAsArrayUnique
		elixir?:         [...string] | string
		rust?:           [...string] | string | number
		erlang?:         [...string] | string
		julia?:          #stringOrNumberOrAcceptBothTypeAsArrayUnique
		opt_release?:    [...string] | string
		rvm?:            #stringOrNumberOrAcceptBothTypeAsArrayUnique
		gemfile?:        string | [...string]
		bundler_args?:   string
		r?:              [...string] | string
		pandoc_version?: string

		// A list of packages to install via brew. This option is ignored
		// on non-OS X builds.
		brew_packages?: [...string]
		r_binary_packages?: [...string]
		r_packages?: [...string]
		bioc_packages?: [...string]
		r_github_packages?: [...string]
		apt_packages?: [...string]

		// CRAN mirror to use for fetching packages
		cran?: string

		// Dictionary of repositories to pass to options(repos)
		repos?: {
			[string]: string
		}

		// The CPU Architecture to run the job on
		arch?: "amd64" | "arm64" | "ppc64le" | "s390x" | "arm64-graviton2" | list.UniqueItems() & [_, ...] & [..."amd64" | "arm64" | "ppc64le" | "s390x" | "arm64-graviton2"]

		// The operating system to run the job on
		os?:        "osx" | "linux" | "windows" | list.UniqueItems() & [_, ...] & [..."osx" | "linux" | "windows"]
		osx_image?: #xcodeVersions | list.UniqueItems() & [_, ...] & [...#xcodeVersions] | *"xcode9.4"

		// The Ubuntu distribution to use
		dist?: "precise" | "trusty" | "xenial" | "bionic" | "focal"

		// sudo is deprecated
		sudo?: true | false | "" | "required" | "enabled"
		addons?: {
			// To install packages not included in the default
			// container-based-infrastructure you need to use the APT addon,
			// as sudo apt-get is not available
			apt?: {
				// To update the list of available packages
				update?: bool
				sources?: [...{
					// Key-value pairs which will be added to /etc/apt/sources.list
					sourceline: string

					// When APT sources require GPG keys, you can specify this with
					// key_url
					key_url?: string
				} | string]

				// To install packages from the package whitelist before your
				// custom build steps
				packages?: [...string]
			}

			// If your build requires setting up custom hostnames, you can
			// specify a single host or a list of them. Travis CI will
			// automatically setup the hostnames in /etc/hosts for both IPv4
			// and IPv6.
			hosts?: [...string] | string

			// Travis CI can add entries to ~/.ssh/known_hosts prior to
			// cloning your git repository, which is necessary if there are
			// git submodules from domains other than github.com,
			// gist.github.com, or ssh.github.com.
			ssh_known_hosts?: #stringOrStringArrayUnique
			artifacts?:       true | {
				s3_region?: string
				paths?: [...string]

				// If you’d like to upload file from a specific directory, you can
				// change your working directory
				working_dir?: string

				// If you’d like to see more detail about what the artifacts addon
				// is doing
				debug?: bool
				...
			}

			// Firefox addon
			firefox?: ("latest" | "latest-esr" | "latest-beta" | "latest-dev" | "latest-nightly" | "latest-unsigned" | #nonEmptyString) & _

			// Chrome addon
			chrome?: "stable" | "beta"

			// RethinkDB addon
			rethinkdb?: string

			// PostgreSQL addon
			postgresql?: string

			// MariaDB addon
			mariadb?: string

			// Sauce Connect addon
			sauce_connect?: {
				username?:   string
				access_key?: string
				...
			} | bool

			// SonarCloud addon
			sonarcloud?: {
				organization?: string
				token?:        #secretString
				...
			}

			// Coverity Scan addon
			coverity_scan?: {
				// GitHub project metadata
				project?: {
					name:         string
					version?:     number
					description?: string
					...
				}

				// Where email notification of build analysis results will be sent
				notification_email?: string

				// Commands to prepare for build_command
				build_command_prepend?: string

				// The command that will be added as an argument to 'cov-build' to
				// compile your project for analysis
				build_command?: string

				// Pattern to match selecting branches that will run analysis. We
				// recommend leaving this set to 'coverity_scan'
				branch_pattern?: string
				...
			}

			// Homebrew addon
			homebrew?: {
				taps?:     #stringOrStringArrayUnique
				packages?: #stringOrStringArrayUnique
				casks?:    #stringOrStringArrayUnique
				brewfile?: #nonEmptyString | (bool | *true)
				update?:   bool | *true
			}

			// SourceClear addon
			srcclr?: bool | *true | {
				debug?: bool | *true
			}

			// Snaps addon
			snaps?: #nonEmptyString | list.UniqueItems() & [_, ...] & [...#nonEmptyString | {
				name:     #nonEmptyString
				channel?: #nonEmptyString

				// 'classic:' is deprecated, use 'confinement:'
				classic?:     bool
				confinement?: "classic" | "devmode"
			}]

			// BrowserStack addon
			browserstack?: {
				username?:   #nonEmptyString
				access_key?: #possiblySecretString
				app_path?:   #nonEmptyString
				proxyHost?:  #nonEmptyString
				proxyPort?:  #nonEmptyString
				proxyUser?:  #nonEmptyString
				proxyPass?:  #nonEmptyString
				forcelocal?: bool
				only?:       #nonEmptyString
				...
			}
		}
		cache?: false | #cache | [...#cache | {
			directories?: [...string]
		}] | {
			directories?: [...string]

			// Upload timeout in seconds
			timeout?:   number | *1800
			apt?:       bool
			bundler?:   bool
			cocoapods?: bool
			pip?:       bool
			yarn?:      bool
			ccache?:    bool
			packages?:  bool
			cargo?:     bool
			npm?:       bool
		}
		services?: #service | [...#service]
		git?: {
			depth?: int | *50 | false

			// Travis CI clones repositories without the quiet flag (-q) by
			// default. Enabling the quiet flag can be useful if you’re
			// trying to avoid log file size limits or even if you just don’t
			// need to include it.
			quiet?: bool

			// Control whether submodules should be cloned
			submodules?: bool

			// Skip fetching the git-lfs files during the initial git clone
			// (equivalent to git lfs smudge --skip),
			lfs_skip_smudge?: bool

			// In some work flows, like build stages, it might be beneficial
			// to skip the automatic git clone step.
			clone?: bool

			// Is a path to the existing file in the current repository with
			// data you’d like to put into $GIT_DIR/info/sparse-checkout file
			// of format described in Git documentation.
			sparse_checkout?: #nonEmptyString

			// Specify handling of line endings when cloning repository
			autocrlf?: bool | "input"
		}

		// Specify which branches to build
		branches?: {
			except?: [...string]
			only?: [...string]
		}
		env?: #envVars | {
			global?: #envVars
			matrix?: #envVars
			jobs?:   #envVars
		}
		before_install?: #step
		install?:        #step
		before_script?:  #step
		script?:         #step
		before_cache?:   #step
		after_success?:  #step
		after_failure?:  #step
		before_deploy?:  #step
		deploy?:         [...#deployment] | #deployment
		after_deploy?:   #step
		after_script?:   #step
		...
	}

	#deployment: {
		on?: {
			// Tell Travis CI to only deploy on tagged commits
			tags?:         bool | string
			branch?:       string
			all_branches?: bool

			// After your tests ran and before the release, Travis CI will
			// clean up any additional files and changes you made. Maybe that
			// is not what you want, as you might generate some artifacts
			// that are supposed to be released, too.
			skip_cleanup?: bool
			repo?:         string

			// if [[ <condition> ]]; then <deploy>; fi
			condition?: string
			...
		}
		...
	} & ({
		provider: "script"
		script:   string
		...
	} | ({
		provider: _
		email:    _
		api_key:  _
		...
	} | {
		provider:  _
		email:     _
		api_token: _
		...
	}) & {
		provider?:  "npm"
		email?:     #possiblySecretString
		api_key?:   #possiblySecretString
		api_token?: #possiblySecretString
		tag?:       string
		...
	} | {
		provider: "surge"
		project?: string
		domain?:  string
		...
	} | {
		provider:   "releases"
		api_key?:   #possiblySecretString
		user?:      #possiblySecretString
		password?:  #possiblySecretString
		file?:      string | [...string]
		file_glob?: bool

		// If you need to overwrite existing files
		overwrite?: bool
		...
	} | {
		provider: "heroku"

		// heroku auth token
		api_key: (#possiblySecretString | {
			[string]: #possiblySecretString
		}) & _
		app?: string | {
			[string]: string
		}

		// to run a command on Heroku after a successful deploy
		run?: string | [...string]

		// Travis CI default will clean up any additional files and
		// changes you made, you can by it to skip the clean up
		skip_cleanup?: bool

		// Travis CI supports different mechanisms for deploying to
		// Heroku: api is default
		strategy?: "api" | "git"
		...
	} | {
		provider:              "s3"
		access_key_id:         #possiblySecretString
		secret_access_key:     #possiblySecretString
		bucket:                string
		region?:               string
		skip_cleanup?:         bool | *false
		acl?:                  "private" | "public_read" | "public_read_write" | "authenticated_read" | "bucket_owner_read" | "bucket_owner_full_control"
		local_dir?:            string
		"upload-dir"?:         string
		detect_encoding?:      bool | *false
		default_text_charset?: string
		cache_control?:        string
		expires?:              string
		endpoint?:             string
		...
	} | {
		provider: string
		...
	})

	#notificationObject: _

	#import: ({
		// The source to import build config from
		source: #nonEmptyString

		// How to merge the imported config into the target config
		// (defaults to deep_merge_append)
		mode?: "merge" | "deep_merge" | "deep_merge_append" | "deep_merge_prepend"

		// Specifies a condition for the import
		if?: #nonEmptyString
	} | #nonEmptyString) & _
}
