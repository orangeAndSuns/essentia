Pod::Spec.new do |spec|
  spec.name         = 'Gess'
  spec.version      = '{{.Version}}'
  spec.license      = { :type => 'GNU Lesser General Public License, Version 3.0' }
  spec.homepage     = 'https://github.com/orangeAndSuns/essentia'
  spec.authors      = { {{range .Contributors}}
		'{{.Name}}' => '{{.Email}}',{{end}}
	}
  spec.summary      = 'iOS Essentia Client'
  spec.source       = { :git => 'https://github.com/orangeAndSuns/go-essentia.git', :commit => '{{.Commit}}' }

	spec.platform = :ios
  spec.ios.deployment_target  = '9.0'
	spec.ios.vendored_frameworks = 'Frameworks/Gess.framework'

	spec.prepare_command = <<-CMD
    curl https://gessstore.blob.core.windows.net/builds/{{.Archive}}.tar.gz | tar -xvz
    mkdir Frameworks
    mv {{.Archive}}/Gess.framework Frameworks
    rm -rf {{.Archive}}
  CMD
end
