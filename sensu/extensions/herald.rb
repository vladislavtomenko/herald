#!/opt/sensu/embedded/bin/ruby
require 'sensu-handler'
require 'net/http'
require "rubygems"
require "json"

class Show < Sensu::Handler
  def handle
    case @event['check']['status']
    when 0
      #Enable service after check recovery
      urlEnable = "http://#{heraldHost}:#{heraldPort}/enable"
      uriEnable = URI("#{urlEnable}")
      request = Net::HTTP::Get.new(uriEnable.path)
      request.basic_auth "#{heraldUser}", "#{heraldPass}"
      response = Net::HTTP.start(uriStatus.host,uriStatus.port) do |http|
      http.request(request)
    end
        exit 0
    when 1
        serviceState='warning'
    when 2
        serviceState='critical'
    else
        serviceState='unknown'
    end

    #check parameters
    hostName=@event['client']['name']
    serviceName=@event['check']['name']
    alertMessage=@event['check']['output']
    
    #herald connection parameters
    heraldUser=settings.to_hash['herald']['user']
    heraldPass=settings.to_hash['herald']['pass']
    heraldHost=settings.to_hash['herald']['host']
    heraldPort=settings.to_hash['herald']['port']

    #get herald service status
    urlStatus = "http://#{heraldHost}:#{heraldPort}/status"
    uriStatus = URI("#{urlStatus}")
    request = Net::HTTP::Get.new(uriStatus.path)
    request.basic_auth "#{heraldUser}", "#{heraldPass}"
    response = Net::HTTP.start(uriStatus.host,uriStatus.port) do |http|
      http.request(request)
    end
    
    parsedResponse = JSON.parse(response.body)
    if parsedResponse["status"] == 'ok'
      #make call
      urlCall = "http://#{heraldHost}:#{heraldPort}/call"
      uriCall = URI("#{urlCall}")
      request = Net::HTTP::Post.new(uriCall.path)
      request.basic_auth "#{heraldUser}", "#{heraldPass}"
      request.set_form_data({"host" => "#{hostName}", "service" => "#{serviceName}", "state" => "#{serviceState}", "message" => "#{alertMessage}"})
      response = Net::HTTP.start(uriCall.host,uriCall.port) do |http|
	http.request(request)
      end

      #disable for 5 minutes
      urlDisable = "http://#{heraldHost}:#{heraldPort}/ack"
      uriDisable = URI("#{urlDisable}")
      request = Net::HTTP::Post.new(uriDisable.path)
      request.basic_auth "#{heraldUser}", "#{heraldPass}"
      request.set_form_data({"duration" => "300"})
      response = Net::HTTP.start(uriStatus.host,uriStatus.port) do |http|
        http.request(request)
      end
    end
    
  end
end
